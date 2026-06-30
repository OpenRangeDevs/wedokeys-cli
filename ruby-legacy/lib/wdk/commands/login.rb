require_relative "../config"
require_relative "../client"

module Wdk
  module Commands
    class Login
      def initialize(options, home_dir: Dir.home, start_dir: Dir.pwd)
        @options   = options
        @home_dir  = home_dir
        @start_dir = start_dir
      end

      def run
        token = @options["token"] || prompt_for_token

        config = Config.new(home_dir: @home_dir, start_dir: @start_dir)
        client = Client.new(api_url: config.api_url, token: token)

        verify_token!(client)
        config.save!(token: token)

        puts "Logged in. Token saved to ~/.wedokeys/config.yml"
      end

      private

      def prompt_for_token
        $stderr.print "Paste your wedokeys service account token: "
        token = $stdin.gets.to_s.strip
        abort "Token cannot be blank." if token.empty?
        token
      end

      def verify_token!(client)
        client.resolve_by_aliases(aliases: [], project: "__verify__", environment: "development")
      rescue Client::AuthError
        abort "Invalid token — authentication failed."
      rescue Client::NetworkError => e
        abort "Network error: #{e.message}"
      rescue Client::ApiError => e
        # The probe is deliberately invalid, so a 400 means the server authenticated
        # us and rejected the params — i.e. the token is valid. Any other status
        # (5xx, unexpected) is not a verification, so don't save the token.
        return if e.status == 400

        abort "Could not verify token (server error #{e.status}). Please try again."
      end
    end
  end
end
