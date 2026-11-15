# Rendered from the immutable archive hashes by make snapshot/package-release.
class PlaneCli < Formula
  desc "JSON-first CLI for the Plane public API"
  homepage "https://github.com/foae/plane-cli"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/foae/plane-cli/releases/download/v@VERSION@/plane-cli_@VERSION@_darwin_arm64.tar.gz"
      sha256 "@SHA_DARWIN_ARM64@"
    end
    on_intel do
      url "https://github.com/foae/plane-cli/releases/download/v@VERSION@/plane-cli_@VERSION@_darwin_amd64.tar.gz"
      sha256 "@SHA_DARWIN_AMD64@"
    end
  end
  on_linux do
    on_arm do
      url "https://github.com/foae/plane-cli/releases/download/v@VERSION@/plane-cli_@VERSION@_linux_arm64.tar.gz"
      sha256 "@SHA_LINUX_ARM64@"
    end
    on_intel do
      url "https://github.com/foae/plane-cli/releases/download/v@VERSION@/plane-cli_@VERSION@_linux_amd64.tar.gz"
      sha256 "@SHA_LINUX_AMD64@"
    end
  end

  def install
    bin.install "plane-cli"
  end

  test do
    require "json"
    require "socket"
    require "timeout"
    assert_equal "1", JSON.parse(shell_output("#{bin}/plane-cli version"))["schema_version"].to_s
    server = TCPServer.new("127.0.0.1", 0)
    port = server.addr[1]
    pid = fork do
      Timeout.timeout(10) do
        socket = server.accept
        request = socket.gets
        raise "unexpected route: #{request}" if request != "GET /api/v2/workspaces/test/projects/ HTTP/1.1\r\n"

        headers = []
        while (line = socket.gets) && line != "\r\n"
          headers << line
        end
        raise "unexpected auth" if headers.any? { |line| line.downcase.start_with?("authorization:", "x-api-key:") }

        body = JSON.generate({
          "results" => [{ "id" => "project-fixture", "name" => "Packaged CLI" }],
          "next_cursor" => nil, "next_page_results" => false
        })
        socket.write "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\n" \
                     "Content-Length: #{body.bytesize}\r\nConnection: close\r\n\r\n#{body}"
        socket.close
      end
      exit! 0
    rescue => e
      warn e.message
      exit! 1
    end
    server.close
    begin
      ENV["PLANE_CLI_CONFIG_DIR"] = (testpath/"config").to_s
      command = "#{bin}/plane-cli project list --api-version v2 --auth-mode external " \
                "--base-url http://127.0.0.1:#{port} --workspace test"
      result = JSON.parse(shell_output(command))
      assert_equal "project-fixture", result.fetch("data").fetch("results").first.fetch("id")
      status = Timeout.timeout(12) { Process.wait2(pid).last }
      pid = nil
      assert_predicate status, :success?
    ensure
      begin
        Process.kill("TERM", pid) if pid
      rescue Errno::ESRCH
        nil
      end
      begin
        Process.wait(pid) if pid
      rescue Errno::ECHILD
        nil
      end
    end
  end
end
