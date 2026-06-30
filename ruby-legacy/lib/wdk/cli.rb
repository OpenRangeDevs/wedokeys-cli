require "thor"
require_relative "version"
require_relative "config"
require_relative "client"
require_relative "commands/login"
require_relative "commands/shell"
require_relative "commands/env"
require_relative "commands/kamal_fetch"

module Wdk
  class CLI < Thor
    desc "login", "Authenticate with wedokeys (paste your service account token)"
    method_option :token, type: :string, desc: "Token to store (skips interactive prompt)"
    def login
      Commands::Login.new(options).run
    end

    desc "subshell", "Open a sub-shell with secrets loaded into the environment"
    method_option :env, type: :string, aliases: "-e", desc: "Environment (overrides WDK_ENV / KAMAL_DESTINATION)"
    method_option :allow_missing, type: :boolean, default: false, desc: "Proceed even if some aliases fail to resolve"
    def subshell
      Commands::Shell.new(options).run
    end

    desc "env SUBCOMMAND", "Manage secret injection"
    subcommand "env", Commands::Env

    desc "kamal-fetch", "Kamal secrets adapter — fetch secrets by alias (internal use)"
    method_option :account, type: :string, desc: "Ignored; account is derived from token"
    method_option :from,    type: :string, desc: "project/environment (falls back to wdk.yml + env inference)"
    method_option :env,     type: :string, aliases: "-e", desc: "Environment override"
    method_option :allow_missing, type: :boolean, default: false, desc: "Proceed even if some aliases fail to resolve"
    def kamal_fetch(*alias_names)
      Commands::KamalFetch.new(options, alias_names).run
    end

    desc "version", "Print the wdk version"
    map %w[--version -v] => :version
    def version
      puts "wdk #{Wdk::VERSION}"
    end

    def self.exit_on_failure?
      true
    end
  end
end
