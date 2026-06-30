require_relative "../config"
require_relative "../client"
require_relative "../resolve_guard"

module Wdk
  module Commands
    class KamalFetch
      def initialize(options, alias_names, home_dir: Dir.home, start_dir: Dir.pwd)
        @options     = options
        @alias_names = alias_names
        @home_dir    = home_dir
        @start_dir   = start_dir
      end

      def run
        if @alias_names.empty?
          $stderr.puts "Error: at least one secret name is required."
          exit 1
        end

        config  = Config.new(home_dir: @home_dir, start_dir: @start_dir, env_option: @options["env"])
        project, environment = resolve_project_and_env(config)
        client  = Client.new(api_url: config.api_url, token: config.token!)

        result = client.resolve_by_aliases(
          aliases:     @alias_names,
          project:     project,
          environment: environment
        )

        # Kamal expects every requested secret; missing ones must fail the
        # fetch, not surface later as absent env vars in a deployed app.
        ResolveGuard.check!(result, allow_missing: @options["allow_missing"])

        # Kamal parses our output as one NAME=value per line. A value that spans
        # multiple lines (PEM key, multiline string) or a whole-secret hash (a
        # "*" reference) can't be expressed that way; escaping a newline would
        # corrupt the secret, so we fail hard with a clear message instead.
        assert_single_line_values!(result[:resolved])

        result[:resolved].each do |name, value|
          puts "#{name}=#{value}"
        end
      rescue Client::AuthError => e
        $stderr.puts "Authentication error: #{e.message}"
        exit 1
      rescue Client::NetworkError => e
        $stderr.puts "Network error: #{e.message}"
        exit 1
      rescue Client::ApiError => e
        $stderr.puts "API error: #{e.message}"
        exit 1
      rescue Config::NotConfiguredError, Config::MissingProjectError, Config::MissingEnvironmentError => e
        $stderr.puts "Configuration error: #{e.message}"
        exit 1
      end

      private

      def assert_single_line_values!(resolved)
        offenders = resolved.reject { |_name, value| value.is_a?(String) && !value.match?(/[\r\n]/) }
        return if offenders.empty?

        offenders.each_key do |name|
          $stderr.puts "#{name}: value is not a single line (multiline secret or whole-secret \"*\" reference); " \
                       "reference a specific field instead."
        end
        abort "Error: #{offenders.size} secret(s) cannot be emitted as NAME=value lines for Kamal."
      end

      def resolve_project_and_env(config)
        from = @options["from"]
        if from&.include?("/")
          project, env = from.split("/", 2)
          [ project, env ]
        elsif from
          [ from, config.environment! ]
        else
          [ config.project_slug!, config.environment! ]
        end
      end
    end
  end
end
