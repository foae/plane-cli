#!/usr/bin/env ruby
# Invoked only by check-installation.sh with its copied Ruby and empty environment.
require "digest"
require "fileutils"
require "json"
require "open3"
require "rbconfig"
require "socket"
require "timeout"

abort "usage: check-formula.rb ARTIFACT_DIR TEMP_DIR HOST_OS HOST_ARCH" unless ARGV.length == 4
artifacts, temporary, host_os, host_arch = ARGV
prefix = File.join(temporary, "prefix")
brew = File.join(prefix, "bin", "brew")
formula_name = "plane/acceptance/plane-cli"
formula_path = File.join(prefix, "Homebrew/Library/Taps/plane/homebrew-acceptance/Formula/plane-cli.rb")

def run!(*command)
  puts "$ #{command.join(' ')}"
  system(*command, exception: true)
end

# Refuse to operate if Homebrew resolves outside the throwaway prefix/repository.
{"--prefix" => prefix, "--repository" => File.join(prefix, "Homebrew")}.each do |flag, expected|
  output, status = Open3.capture2(brew, flag)
  abort "Homebrew isolation failed for #{flag}: #{output}" unless status.success? && output.strip == expected
end

metadata = JSON.parse(File.read(File.join(artifacts, "release-metadata.json")))
version = File.read("VERSION").strip
abort "staged version differs from VERSION" unless metadata.fetch("version") == version
archive_names = %w[linux_amd64 linux_arm64 darwin_amd64 darwin_arm64].map do |target|
  "plane-cli_#{version}_#{target}.tar.gz"
end
expected_names = archive_names + ["plane-cli-skill_#{version}.tar.gz", "plane-cli.rb", "release-metadata.json"]
checksums = {}
File.foreach(File.join(artifacts, "SHA256SUMS")) do |line|
  match = /\A([0-9a-f]{64})  ([^\/\s]+)\n?\z/.match(line)
  abort "invalid checksum line: #{line.inspect}" unless match
  sum, name = match.captures
  abort "unexpected or duplicate checksum member: #{name}" unless expected_names.include?(name) && !checksums.key?(name)
  abort "checksum mismatch: #{name}" unless Digest::SHA256.file(File.join(artifacts, name)).hexdigest == sum
  checksums[name] = sum
end
abort "manifest must cover all seven staged members" unless checksums.keys.sort == expected_names.sort
archive_checksums = checksums.reject { |name, _| !name.end_with?(".tar.gz") }
abort "metadata archive hashes differ" unless metadata.fetch("archive_sha256") == archive_checksums

%w[version schema-v1 schema-v2].each do |operation|
  output = JSON.parse(File.read(File.join(temporary, "go-#{operation}.out")))
  abort "go install #{operation} envelope mismatch" unless output.fetch("schema_version").to_s == "1"
end
abort "go install help is empty" if File.read(File.join(temporary, "go---help.out"), encoding: "UTF-8").strip.empty?
puts "Isolated go install: version, help and v1/v2 schemas passed"

FileUtils.mkdir_p(File.dirname(formula_path))
FileUtils.cp(File.join(artifacts, "plane-cli.rb"), formula_path)
run!("git", "init", "--quiet", File.dirname(File.dirname(formula_path)))
run!(RbConfig.ruby, "-c", formula_path)
# Only this generated, isolated tap is trusted; the caller's trust store is untouched.
run!(brew, "trust", "plane/acceptance")
# Audit the actual production URLs, not localhost exceptions. The temporary tap
# is not Homebrew/core and does not claim core's stable-only admission policy.
run!(brew, "audit", "--os=all", "--arch=all", "--formula", formula_name)

server = TCPServer.new("127.0.0.1", 0)
port = server.addr[1]
request_log = File.join(temporary, "http-requests.log")
File.write(request_log, "")
routes = archive_names.to_h { |name| ["/v#{version}/#{name}", File.join(artifacts, name)] }
pid = nil
Signal.trap("TERM") { raise Interrupt }
Signal.trap("HUP") { raise Interrupt }
begin
  # Binding before fork makes the server ready without a sleep or a fixed port.
  pid = fork do
    begin
      loop do
        socket = server.accept
        begin
          Timeout.timeout(15) do
            method, route, protocol = socket.gets.to_s.split
            while (header = socket.gets) && header != "\r\n"
              # Consume request headers; staged assets need no authentication.
            end
            path = routes[route]
            if %w[GET HEAD].include?(method) && protocol == "HTTP/1.1" && path
              File.open(request_log, "a") { |log| log.puts "#{method} #{route}" }
              socket.write "HTTP/1.1 200 OK\r\nContent-Type: application/gzip\r\nContent-Length: #{File.size(path)}\r\nConnection: close\r\n\r\n"
              File.open(path, "rb") { |file| IO.copy_stream(file, socket) } if method == "GET"
            else
              socket.write "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
            end
          end
        rescue Errno::EPIPE, Errno::ECONNRESET
          # A client can close a probe/download early; later requests still need service.
          next
        ensure
          socket.close
        end
      end
    rescue Interrupt
      exit! 0
    rescue StandardError => error
      warn "staged HTTP server: #{error.message}"
      exit! 1
    end
  end
  server.close
  production_base = "https://github.com/foae/plane-cli/releases/download/"
  formula = File.read(formula_path)
  abort "formula must contain exactly four production URLs" unless formula.scan(production_base).length == 4
  local_formula = formula.gsub(production_base, "http://127.0.0.1:#{port}/")
  File.write(formula_path, local_formula)
  run!(RbConfig.ruby, "-c", formula_path)
  run!(brew, "install", "--build-from-source", "--formula", formula_name)
  run!(brew, "test", formula_name)
  binary = File.join(prefix, "bin", "plane-cli")
  output, status = Open3.capture2({"PLANE_CLI_CONFIG_DIR" => File.join(temporary, "brew-config")}, binary, "version")
  abort "installed Homebrew version failed" unless status.success?
  identity = JSON.parse(output).fetch("data")
  unless identity.fetch("version").delete_prefix("v") == metadata.fetch("cli_version") && identity.fetch("commit") == metadata.fetch("commit")
    abort "installed Homebrew CLI identity differs from release metadata"
  end
  expected_request = "GET /v#{version}/plane-cli_#{version}_#{host_os}_#{host_arch}.tar.gz"
  abort "Homebrew did not fetch the host archive from the controlled server" unless File.readlines(request_log, chomp: true).include?(expected_request)
  puts File.read(request_log)
  puts "Homebrew syntax, four-target audit, staged HTTP install and formula test passed (#{host_os}/#{host_arch})"
ensure
  server.close unless server.closed?
  if pid
    begin
      Process.kill("TERM", pid)
    rescue Errno::ESRCH
      nil
    end
    begin
      Process.wait(pid)
    rescue Errno::ECHILD
      nil
    end
  end
end
