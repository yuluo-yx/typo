class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.8.2"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.2/typo-darwin-arm64", using: :nounzip
    sha256 "ee4ca12ae471f6e7df38d2799b07b3ee53685e80616925dee3920e771748e552"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.2/typo-darwin-amd64", using: :nounzip
    sha256 "deda437e559a14309a91be66a652dcf612ea03c67fc302560436cb479507f279"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.2/typo-linux-arm64", using: :nounzip
    sha256 "a3cb3827b00c45a27e992813ea8ef910c31dd019d03b9cdfcae6fb45c39df0dc"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.8.2/typo-linux-amd64", using: :nounzip
    sha256 "7c3c934a4c53b005813e047422ca6c8131716b36de14d25ed764d164ae55d0bc"
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
