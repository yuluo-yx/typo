class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.3/typo-darwin-arm64", using: :nounzip
    sha256 "64393c7c2a8323384f174109e0f492b316c7f009ac85a03c1d4c10eefffab72b"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.3/typo-darwin-amd64", using: :nounzip
    sha256 "c9c729d7b7c27ceee31fb5358069b8c490e653185bdd667c0d771be9a683b38e"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.3/typo-linux-arm64", using: :nounzip
    sha256 "b0a349275e95a130ea0f3108f7dfb1e60bae6182b00daf3cf5a304b0734be571"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.3/typo-linux-amd64", using: :nounzip
    sha256 "31d5bf14154445cf2c9e4a6a6904f0dba996188fc32f8b35afca59abe2b6e51f"
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
