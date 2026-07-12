class Typo < Formula
  desc "Command auto-correction tool"
  homepage "https://github.com/yuluo-yx/typo"
  version "1.7.0"
  license "MIT"

  if OS.mac? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.0/typo-darwin-arm64", using: :nounzip
    sha256 "b96d11d3cef0cbdcb98c21e5974a5ff647fd505c960658dfbc18f6e0f9e0ce3b"
  elsif OS.mac?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.0/typo-darwin-amd64", using: :nounzip
    sha256 "3ff90656655505897c5d905cc74048f8a99545c8221649c47de1f9d6f9ddf4d8"
  elsif OS.linux? && Hardware::CPU.arm?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.0/typo-linux-arm64", using: :nounzip
    sha256 "b35fb57b687c4e3613c09516ea51514d8e0614d9f29dbc4d59c1b2f6faa967a0"
  elsif OS.linux?
    url "https://github.com/yuluo-yx/typo/releases/download/v1.7.0/typo-linux-amd64", using: :nounzip
    sha256 "17c427f47ed3a4f63c452682c722ec19dcb43e6c9b45fb40b76b9d04ed2318ef"
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
