require "yaml"
require "fileutils"

module Wdk
  class Config
    class NotConfiguredError  < StandardError; end
    class MissingProjectError < StandardError; end
    class MissingEnvironmentError < StandardError; end

    DEFAULT_API_URL = "https://app.wedokeys.com"

    def initialize(home_dir: Dir.home, start_dir: Dir.pwd, env_option: nil)
      @home_dir   = home_dir
      @start_dir  = start_dir
      @env_option = env_option
    end

    # --- User config (~/.wedokeys/config.yml) ---

    def token
      user_config["token"]
    end

    def token!
      token || raise(NotConfiguredError, "No token found. Run `wdk login` first.")
    end

    def api_url
      user_config["api_url"] || DEFAULT_API_URL
    end

    # Merges over the existing file so settings like api_url survive a
    # re-login instead of being wiped.
    def save!(token:, api_url: nil)
      data = user_config.merge("token" => token)
      data["api_url"] = api_url if api_url
      FileUtils.mkdir_p(File.dirname(config_file_path))
      File.write(config_file_path, YAML.dump(data))
      File.chmod(0o600, config_file_path)
    end

    # --- Project config (wdk.yml) ---

    def project_slug
      project_config&.fetch("project", nil)
    end

    def project_slug!
      project_slug || raise(MissingProjectError,
        "No wdk.yml found. Create one in your project root with:\n  project: <slug>")
    end

    def secrets
      project_config&.fetch("secrets", nil) || []
    end

    # --- Environment resolution ---

    def environment
      nonempty(@env_option) ||
        nonempty(ENV["WDK_ENV"]) ||
        nonempty(ENV["KAMAL_DESTINATION"])
    end

    def environment!
      environment || raise(MissingEnvironmentError,
        "Environment not set. Pass --env, or set WDK_ENV / KAMAL_DESTINATION.")
    end

    private

    def user_config
      @user_config ||= File.exist?(config_file_path) ? YAML.safe_load_file(config_file_path) || {} : {}
    end

    def config_file_path
      File.join(@home_dir, ".wedokeys", "config.yml")
    end

    def project_config
      return @project_config if defined?(@project_config)
      path = find_wdk_yml
      @project_config = path ? (YAML.safe_load_file(path) || {}) : nil
    end

    def nonempty(val)
      val && !val.empty? ? val : nil
    end

    def find_wdk_yml
      dir = File.expand_path(@start_dir)
      loop do
        candidate = File.join(dir, "wdk.yml")
        return candidate if File.exist?(candidate)
        parent = File.dirname(dir)
        break if parent == dir
        dir = parent
      end
      nil
    end
  end
end
