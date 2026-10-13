# This project tap distributes prebuilt binaries from tagged releases.
class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.4/typo-darwin-arm64", using: :nounzip
    sha256 "e5ecf6a30229faaa0ad1489dd46e1296871856d426053751298755f11a86e48d"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.4/typo-darwin-amd64", using: :nounzip
    sha256 "32cd4f082c24d0b3728a8e64d6cfdf7d0b7b5fa467375a42b3239b405ce0a6b4"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.4/typo-linux-arm64", using: :nounzip
    sha256 "47711b45d0cc77776b97f972c913e15d88062d0fb798338771f402ca5621930c"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.4/typo-linux-amd64", using: :nounzip
    sha256 "be72712ea6ac5272683c530a29f4a03840d6dcaa89f4c400739fae611e11d5ec"
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
