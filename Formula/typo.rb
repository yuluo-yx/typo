class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.7.1"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.1/typo-darwin-arm64", using: :nounzip
    sha256 "a688db546c8bf516677f09f5df379ca60b71fe36dd6a85607b76c75721352b8e"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.1/typo-darwin-amd64", using: :nounzip
    sha256 "ac48a2d5a407b93571b0c82b4650f293f2d7a4ceeb3c4c7b34416105d0ce6721"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.1/typo-linux-arm64", using: :nounzip
    sha256 "a669608a8896ff84fc03ff98ac14804b313f2868440c711a036a224f48b0e1be"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.1/typo-linux-amd64", using: :nounzip
    sha256 "a6beba68b1107a132dcfa76d82981c74aac0e0b7d52d17efba79e58feec1e19d"
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
