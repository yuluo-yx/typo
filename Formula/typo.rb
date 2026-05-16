class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.4.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.4.0/typo-darwin-arm64", using: :nounzip
    sha256 "1680776f7e0ca745a3fb86083f32b7397785c546009c14ae8bad4a114ea6cf1e"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.4.0/typo-darwin-amd64", using: :nounzip
    sha256 "e2c91d587dcdf1d5d65cd1d5eee3cfb0e3df6977fbf3107d919bc287847cf1d3"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.4.0/typo-linux-arm64", using: :nounzip
    sha256 "b866e894612a7d02229262ba3ce8f5abdc448eb22d058d3188dd830ca59dd817"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.4.0/typo-linux-amd64", using: :nounzip
    sha256 "218e82a628b91c29ac9799b577cf979999a81d46c40b19b93de45e63796d3088"
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
