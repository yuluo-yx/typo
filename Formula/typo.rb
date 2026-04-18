class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.0.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.0.0/typo-darwin-arm64", using: :nounzip
    sha256 "cf454b2138dc2084c10256eb2cf3a4c0333e075938a393adbac0d7404fbc3006"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.0.0/typo-darwin-amd64", using: :nounzip
    sha256 "d2ede7f03050e42bb708dc2753e29faedb88e2707840af36ca85c1defa22982b"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.0.0/typo-linux-arm64", using: :nounzip
    sha256 "80cc1fcf65a232b8c16f6c6e0ace92fa7dd6e91a629334e3031b00a57182b50e"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.0.0/typo-linux-amd64", using: :nounzip
    sha256 "6619004cdadb86adf35ae2b0424f874bed3d897530a007e56981f46de182d53c"
  end

  def install
    binary = Dir["typo-*"].find { |path| File.file?(path) }
    odie "Release binary was not downloaded" if binary.nil?

    chmod 0755, binary
    bin.install binary => "typo"
  end

  test do
    assert_match "typo #{version}", shell_output("#{bin}/typo version")
    assert_equal "git status", shell_output("#{bin}/typo fix 'gut status'").strip
  end
end
