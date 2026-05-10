class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.3.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.3.0/typo-darwin-arm64", using: :nounzip
    sha256 "574e9490cc4017e34eff19af1888003a858834f41c2f91f86ea90bbddcd96f60"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.3.0/typo-darwin-amd64", using: :nounzip
    sha256 "27a43933992723bb7a5275b24462cad6eb4450350b829590234a3ce31ce68dca"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.3.0/typo-linux-arm64", using: :nounzip
    sha256 "4c72ce28a89c659e22b298e554ddb19d52ffeec5c05eb2d5d5410da47033a814"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.3.0/typo-linux-amd64", using: :nounzip
    sha256 "29a684471e88acb137276c7340eef60952c97184ec648b27519dfed2f90c050e"
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
