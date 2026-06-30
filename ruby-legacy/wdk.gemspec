require_relative "lib/wdk/version"

Gem::Specification.new do |spec|
  spec.name        = "wdk"
  spec.version     = Wdk::VERSION
  spec.summary     = "WeDoKeys CLI — inject secrets into local dev and Kamal deploys"
  spec.description = "Resolve WeDoKeys secret aliases at runtime and inject them as " \
                     "environment variables for local development, scripts, and Kamal deploys."
  spec.authors     = [ "WeDoKeys" ]
  spec.homepage    = "https://github.com/OpenRangeDevs/wedokeys"
  spec.license     = "MIT"

  spec.metadata = {
    "homepage_uri"    => spec.homepage,
    "source_code_uri" => "https://github.com/OpenRangeDevs/wedokeys/tree/main/cli",
    "bug_tracker_uri" => "https://github.com/OpenRangeDevs/wedokeys/issues"
  }

  spec.files         = Dir["lib/**/*", "bin/*", "README.md", "LICENSE"]
  spec.bindir        = "bin"
  spec.executables   = %w[wdk kamal-secrets-wedokeys]
  spec.require_paths = [ "lib" ]

  spec.required_ruby_version = ">= 3.1"

  spec.add_dependency "thor", "~> 1.3"
end
