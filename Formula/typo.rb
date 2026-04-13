class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "0.2.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v0.2.0/typo-darwin-arm64", using: :nounzip
    sha256 "e1fe20450c2a48e4608afca156e543a9a5d43f6607fa352d941369c87e0f2b65"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v0.2.0/typo-darwin-amd64", using: :nounzip
    sha256 "01334fb2dc7bd4b90cb9e1d7ac0e7fb0136735e3545833bba8cc88df89a80e9b"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v0.2.0/typo-linux-arm64", using: :nounzip
    sha256 "5ba7a9650912b524b90d9debcd81c577b930e6640c3194ac5454c29ece5d8b09"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v0.2.0/typo-linux-amd64", using: :nounzip
    sha256 "4c4e234de8100a690ca42c6d98b4a081af0f3e42ea190075e6753ccf4ef15313"
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
