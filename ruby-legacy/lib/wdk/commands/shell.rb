require_relative "../config"
require_relative "../client"
require_relative "../resolve_guard"

module Wdk
  module Commands
    class Shell
      def initialize(options, home_dir: Dir.home, start_dir: Dir.pwd)
        @options   = options
        @home_dir  = home_dir
        @start_dir = start_dir
      end

      def run
        config      = Config.new(home_dir: @home_dir, start_dir: @start_dir, env_option: @options["env"])
        project     = config.project_slug!
        environment = config.environment!
        aliases     = config.secrets

        if aliases.empty?
          abort "No secrets listed in wdk.yml. Add a `secrets:` list to load secrets into the shell."
        end

        client  = Client.new(api_url: config.api_url, token: config.token!)
        result  = client.resolve_by_aliases(aliases: aliases, project: project, environment: environment)

        ResolveGuard.check!(result, allow_missing: @options["allow_missing"])

        env_vars = result[:resolved]
        shell    = ENV["SHELL"] || "/bin/sh"
        ps1      = "[wdk:#{project}/#{environment}] "

        exec(env_vars.merge("PS1" => ps1), shell)
      rescue Client::AuthError => e
        abort "Authentication error: #{e.message}"
      rescue Client::NetworkError => e
        abort "Network error: #{e.message}"
      rescue Client::ApiError => e
        abort "API error: #{e.message}"
      rescue Config::NotConfiguredError, Config::MissingProjectError, Config::MissingEnvironmentError => e
        abort "Configuration error: #{e.message}"
      end
    end
  end
end
