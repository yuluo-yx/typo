class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.8.1"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.1/typo-darwin-arm64", using: :nounzip
    sha256 "0d5e6214fd1df083614a3a4fe2b59977694872534b34dfc6bce02e1e71e6387f"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.1/typo-darwin-amd64", using: :nounzip
    sha256 "b5ca99becf65a7614bfa2638ae39ed3dd7a2e5484927e07d33c2ff446dbeb6e5"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.1/typo-linux-arm64", using: :nounzip
    sha256 "6f0498c57332d7b9fd72cb7232163d6fd66cc110bf5ac0f2d85d7d8111f6f212"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.1/typo-linux-amd64", using: :nounzip
    sha256 "766a12c0aa5283f8ec17cf148c3609c79c646c7b2780a9764fa7e7d3462f6e15"
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
