# This project tap distributes prebuilt binaries from tagged releases.
class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.2/typo-darwin-arm64", using: :nounzip
    sha256 "8d4557b2a41ac9662f890305887e804b2ccc0133e48b54940a2a954bb5fcdc65"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.2/typo-darwin-amd64", using: :nounzip
    sha256 "adecc1d3e48189384341849cdcc92be0e3fba5c921cbf62c98ca2080323e9202"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.2/typo-linux-arm64", using: :nounzip
    sha256 "a1f7a2ba1373de132a98468ad26720efd7e864f180a34270cfc8581db7f860d6"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.2/typo-linux-amd64", using: :nounzip
    sha256 "68144cbef722fc8315a82830a9d5ad2a301015f3eb626816cdb0dedb92cd9930"
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
