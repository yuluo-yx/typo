# This project tap distributes prebuilt binaries from tagged releases.
class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.1/typo-darwin-arm64", using: :nounzip
    sha256 "cde9aa5c947375116f488ed79ae6fc535a0dd82a61aa3a3ba9717df6969775e0"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.1/typo-darwin-amd64", using: :nounzip
    sha256 "e6993801dacfd88d43b094f9e08f0558a911c54a29a9824a8da1d0e622e6e7e1"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.1/typo-linux-arm64", using: :nounzip
    sha256 "90f32ece5ad9747908854823dc6875660016654379e178f56b5b7e8ab01034fc"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.1/typo-linux-amd64", using: :nounzip
    sha256 "4930043847cf04195560adb886ccad524018bc7714efa18a4fb25f866946f645"
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
