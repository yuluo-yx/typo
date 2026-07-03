class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.6.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.6.0/typo-darwin-arm64", using: :nounzip
    sha256 "4c5c0e1a82d532fd3e2abda320c380189ec4f2858eed5d8b8999908f0dcda873"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.6.0/typo-darwin-amd64", using: :nounzip
    sha256 "3c5450e848ba31160c8f43c75f2f65bf6d513df2ed09303201a6afdeeb9813ff"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.6.0/typo-linux-arm64", using: :nounzip
    sha256 "6f959b2ff6d679ee77643dc403b7b2daa4e46f3022a5a913664cdfe8d060cc08"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.6.0/typo-linux-amd64", using: :nounzip
    sha256 "cd900d8f388cb86b2bb435fa3f394c577c6fd4a9419c7c0c350f3b95945faea8"
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
