# This project tap distributes prebuilt binaries from tagged releases.
class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.5/typo-darwin-arm64", using: :nounzip
    sha256 "c5a28883613a9ce1a6db3b6dc1ac9be3d852a8d43377ff9510adad9a840bceb0"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.5/typo-darwin-amd64", using: :nounzip
    sha256 "123f89339b1a3068e82f85d78842bb6346eb25a15ec2db6d2e6295932223054e"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.5/typo-linux-arm64", using: :nounzip
    sha256 "2571864d11940a8c32b67723a4635762bf176f5a86a0b64b75a3b006c7c056a0"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.5/typo-linux-amd64", using: :nounzip
    sha256 "3c651e55ef54f6e760be5eb975f5cf6562fe08bb8c29957eac6741527de490fb"
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
