class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.2.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.2.0/typo-darwin-arm64", using: :nounzip
    sha256 "b7e7e803298d828024baf652e4aa0f5e7d00ed2abdefa8e9528c663f8dc7cff4"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.2.0/typo-darwin-amd64", using: :nounzip
    sha256 "9a5b54da8fb4c49860fe75de9ef602c135f27dd05446ed6f887abd855d4b1fc8"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.2.0/typo-linux-arm64", using: :nounzip
    sha256 "8bc5055260ef8b5338234c3a97608a4930e0fe88701ebb0a19fdbd522df0d6fe"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.2.0/typo-linux-amd64", using: :nounzip
    sha256 "d08a40a0a0d168e87feeb9637f5703811b2e74f302521bc5e92338267d69aa2f"
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
