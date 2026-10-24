# This project tap distributes prebuilt binaries from tagged releases.
class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.10.0/typo-darwin-arm64", using: :nounzip
    sha256 "007c26997acfa5a3c697e345f8cd8327399dcfdab938fbf384158288f74a7821"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.10.0/typo-darwin-amd64", using: :nounzip
    sha256 "c03e8fec44a7fe1a22526f8fdcecd30141e2c955c2afa46a9885e17ee7f31379"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.10.0/typo-linux-arm64", using: :nounzip
    sha256 "3a0d15d3c81dd417b4ead21f3f1c5878f75d0cf42c4892177a2d2be3f3f25d20"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.10.0/typo-linux-amd64", using: :nounzip
    sha256 "1c63ccd94f9a290330e4b716e6490fc4269cc5cd635ee18722965db03808c35a"
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
