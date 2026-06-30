require "thor"
require "shellwords"
require_relative "../config"
require_relative "../client"
require_relative "../resolve_guard"

module Wdk
  module Commands
    class Env < Thor
      desc "exec -- COMMAND [ARGS]", "Run a command with secrets injected into its environment"
      method_option :env, type: :string, aliases: "-e", desc: "Environment override"
      method_option :allow_missing, type: :boolean, default: false,
        desc: "Proceed even if some aliases fail to resolve"
      def exec(*cmd_args)
        abort "Usage: wdk env exec -- COMMAND [ARGS]" if cmd_args.empty?

        config      = Config.new(env_option: options["env"])
        project     = config.project_slug!
        environment = config.environment!
        aliases     = config.secrets

        abort "No secrets listed in wdk.yml. Add a `secrets:` list." if aliases.empty?

        client  = Client.new(api_url: config.api_url, token: config.token!)
        result  = client.resolve_by_aliases(aliases: aliases, project: project, environment: environment)

        ResolveGuard.check!(result, allow_missing: options["allow_missing"])

        Kernel.exec(result[:resolved], *cmd_args)
      rescue Client::AuthError => e
        abort "Authentication error: #{e.message}"
      rescue Client::NetworkError => e
        abort "Network error: #{e.message}"
      rescue Client::ApiError => e
        abort "API error: #{e.message}"
      rescue Config::NotConfiguredError, Config::MissingProjectError, Config::MissingEnvironmentError => e
        abort "Configuration error: #{e.message}"
      end

      desc "export", "Print secrets as shell export statements (for scripting / direnv)"
      method_option :env, type: :string, aliases: "-e", desc: "Environment override"
      method_option :allow_missing, type: :boolean, default: false,
        desc: "Proceed even if some aliases fail to resolve"
      def export
        config      = Config.new(env_option: options["env"])
        project     = config.project_slug!
        environment = config.environment!
        aliases     = config.secrets

        abort "No secrets listed in wdk.yml." if aliases.empty?

        client = Client.new(api_url: config.api_url, token: config.token!)
        result = client.resolve_by_aliases(aliases: aliases, project: project, environment: environment)

        ResolveGuard.check!(result, allow_missing: options["allow_missing"])

        result[:resolved].each do |name, value|
          puts "export #{name}=#{value.to_s.shellescape}"
        end
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
