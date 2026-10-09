# This project tap distributes prebuilt binaries from tagged releases.
class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.3/typo-darwin-arm64", using: :nounzip
    sha256 "535d806b1c37bdbb18b05088969df045486070ab3d335bc696d701fcd446e993"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.3/typo-darwin-amd64", using: :nounzip
    sha256 "d2e123db7dbdca2e271961b97c3db41325ea67dde555f5597ca2afa8c87cdaf3"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.3/typo-linux-arm64", using: :nounzip
    sha256 "5abe0d89d1d683eb57f39663b4a989ca656b95ce4db412c4bad4c417e935d8a7"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.9.3/typo-linux-amd64", using: :nounzip
    sha256 "fe7b0a3bb714146e0afb20dc356e4b7b4629b2708f9d45bae533f2025300e1cd"
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
