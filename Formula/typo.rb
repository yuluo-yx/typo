class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.8.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.0/typo-darwin-arm64", using: :nounzip
    sha256 "20bed4f739a2cd790d303e1b33d453dc9b4465c1838acb9ae484c09674f1bdee"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.0/typo-darwin-amd64", using: :nounzip
    sha256 "28fb660f6bf7065024f9d05b4e8a664a297d64b6505f2bacf7e6ec7a81417aa0"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.0/typo-linux-arm64", using: :nounzip
    sha256 "6d1f3b6e749cfbc14ad43010b80c7c52c922ba271ac164af956287d427f7f685"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.0/typo-linux-amd64", using: :nounzip
    sha256 "814f836aebd31fb86763025fe0299431e4a6643d90c70318534f35f05c225ac4"
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
