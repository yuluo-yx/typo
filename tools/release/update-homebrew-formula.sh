#!/usr/bin/env bash

set -euo pipefail

tag="${1:-}"
formula_path="${FORMULA_PATH:-Formula/typo.rb}"
checksums_path="${CHECKSUMS_PATH:-bin/checksums.txt}"

ruby - "$tag" "$formula_path" "$checksums_path" <<'RUBY'
ASSETS = %w[
  typo-darwin-amd64
  typo-darwin-arm64
  typo-linux-amd64
  typo-linux-arm64
].freeze

tag, formula_path, checksums_path = ARGV

unless tag&.match?(/\Av\d+\.\d+\.\d+\z/)
  warn "Usage: tools/release/update-homebrew-formula.sh vX.Y.Z"
  exit 1
end

unless File.file?(formula_path)
  warn "Formula file not found: #{formula_path}"
  exit 1
end

unless File.file?(checksums_path)
  warn "Checksum file not found: #{checksums_path}"
  exit 1
end

checksums = {}
File.foreach(checksums_path) do |line|
  hash, file = line.strip.split(/\s+/, 2)
  next unless ASSETS.include?(file)

  unless hash&.match?(/\A[0-9a-f]{64}\z/i)
    warn "Invalid SHA-256 for #{file}: #{hash}"
    exit 1
  end

  checksums[file] = hash.downcase
end

missing = ASSETS.reject { |asset| checksums.key?(asset) }
unless missing.empty?
  warn "Missing Homebrew checksums: #{missing.join(", ")}"
  exit 1
end

version = tag.delete_prefix("v")
formula = File.read(formula_path)

version_pattern = /version "[^"]+"/
unless formula.match?(version_pattern)
  warn "Unable to find version declaration in #{formula_path}"
  exit 1
end
formula = formula.sub(version_pattern, %(version "#{version}"))

formula = formula.gsub(%r{releases/download/v[^/"]+/}, "releases/download/#{tag}/")

ASSETS.each do |asset|
  pattern = /(url\s+"https:\/\/github\.com\/yuluo-yx\/typo\/releases\/download\/#{Regexp.escape(tag)}\/#{Regexp.escape(asset)}",\s+using:\s+:nounzip\s*\n\s*sha256\s+)"[0-9a-f]{64}"/
  matched = false
  formula = formula.sub(pattern) do
    matched = true
    %(#{$1}"#{checksums.fetch(asset)}")
  end

  next if matched

  warn "Unable to update checksum for #{asset} in #{formula_path}"
  exit 1
end

File.write(formula_path, formula)
puts "Updated #{formula_path} to #{tag}"
RUBY

ruby -c "$formula_path" >/dev/null
