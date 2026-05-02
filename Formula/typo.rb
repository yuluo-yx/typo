class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.1.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.1.0/typo-darwin-arm64", using: :nounzip
    sha256 "e1df0fa51bf33e4111626ec7a20564685efdcd37deb1b67cb032976d12b06203"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.1.0/typo-darwin-amd64", using: :nounzip
    sha256 "480c2ab90c586c9d0158139d57d7468ff0c19d16b5fd3695e51424fa7b404b77"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.1.0/typo-linux-arm64", using: :nounzip
    sha256 "dec147edeb92f85f8ace46a5a4e979188d763382f53e14aaa2e5015a641aec1f"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.1.0/typo-linux-amd64", using: :nounzip
    sha256 "02f29fd68424b6a1ac8773a62c151cd69452bb14accad5219fd7677ba604464e"
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
