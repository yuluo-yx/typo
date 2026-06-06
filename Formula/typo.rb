class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.5.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.5.0/typo-darwin-arm64", using: :nounzip
    sha256 "06a975f4335fb6452282b9c3c544beca4321408a1fe7c8446c4f574e796109df"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.5.0/typo-darwin-amd64", using: :nounzip
    sha256 "891dfd46613f3fb955c155ffe610dbc4bebf340dbca8a9ffca15e5c974bb6860"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.5.0/typo-linux-arm64", using: :nounzip
    sha256 "1f1ee56d740b6da7dbecb747b362bfed661677b67b2d5a7fcd8cb585c8088d91"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.5.0/typo-linux-amd64", using: :nounzip
    sha256 "062e43dc5a28c8d9aed192529c42993e744e7841c0e864f1a7866862155ac8ef"
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
