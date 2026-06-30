require "test_helper"
require "wdk/config"

class Wdk::ConfigTest < Minitest::Test
  def setup
    @tmpdir = Dir.mktmpdir
    @home   = File.join(@tmpdir, "home")
    FileUtils.mkdir_p(@home)
    @project_dir = File.join(@tmpdir, "project")
    FileUtils.mkdir_p(@project_dir)
  end

  def teardown
    FileUtils.rm_rf(@tmpdir)
  end

  # --- User config (token + api_url) ---

  def test_loads_token_from_config_file
    write_user_config(token: "wdk_sat_abc123")
    config = Wdk::Config.new(home_dir: @home)
    assert_equal "wdk_sat_abc123", config.token
  end

  def test_loads_api_url_from_config_file
    write_user_config(api_url: "https://example.wedokeys.app")
    config = Wdk::Config.new(home_dir: @home)
    assert_equal "https://example.wedokeys.app", config.api_url
  end

  def test_default_api_url_is_wedokeys_production
    write_user_config(token: "wdk_sat_abc")
    config = Wdk::Config.new(home_dir: @home)
    assert_equal "https://app.wedokeys.com", config.api_url
  end

  def test_raises_when_no_config_file_and_no_token
    config = Wdk::Config.new(home_dir: @home)
    assert_raises(Wdk::Config::NotConfiguredError) { config.token! }
  end

  def test_config_file_has_0600_permissions
    write_user_config(token: "wdk_sat_abc")
    config_path = File.join(@home, ".wedokeys", "config.yml")
    mode = File.stat(config_path).mode & 0o777
    assert_equal 0o600, mode
  end

  # --- save! ---

  def test_save_preserves_existing_api_url
    write_user_config(token: "wdk_sat_old", api_url: "http://localhost:3000")

    config = Wdk::Config.new(home_dir: @home)
    config.save!(token: "wdk_sat_new")

    reloaded = Wdk::Config.new(home_dir: @home)
    assert_equal "wdk_sat_new", reloaded.token
    assert_equal "http://localhost:3000", reloaded.api_url
  end

  def test_save_with_explicit_api_url_overrides_existing
    write_user_config(token: "wdk_sat_old", api_url: "http://localhost:3000")

    config = Wdk::Config.new(home_dir: @home)
    config.save!(token: "wdk_sat_new", api_url: "https://other.example.com")

    reloaded = Wdk::Config.new(home_dir: @home)
    assert_equal "https://other.example.com", reloaded.api_url
  end

  def test_save_on_fresh_home_writes_token_only
    config = Wdk::Config.new(home_dir: @home)
    config.save!(token: "wdk_sat_fresh")

    path = File.join(@home, ".wedokeys", "config.yml")
    assert_equal({ "token" => "wdk_sat_fresh" }, YAML.safe_load_file(path))
    assert_equal 0o600, File.stat(path).mode & 0o777
  end

  # --- Project config (wdk.yml discovery) ---

  def test_discovers_wdk_yml_in_current_dir
    write_project_config(@project_dir, project: "my-app")
    config = Wdk::Config.new(home_dir: @home, start_dir: @project_dir)
    assert_equal "my-app", config.project_slug
  end

  def test_discovers_wdk_yml_in_parent_dir
    subdir = File.join(@project_dir, "app", "controllers")
    FileUtils.mkdir_p(subdir)
    write_project_config(@project_dir, project: "parent-app")
    config = Wdk::Config.new(home_dir: @home, start_dir: subdir)
    assert_equal "parent-app", config.project_slug
  end

  def test_returns_nil_project_slug_when_no_wdk_yml
    config = Wdk::Config.new(home_dir: @home, start_dir: @project_dir)
    assert_nil config.project_slug
  end

  def test_raises_when_project_slug_required_but_missing
    config = Wdk::Config.new(home_dir: @home, start_dir: @project_dir)
    assert_raises(Wdk::Config::MissingProjectError) { config.project_slug! }
  end

  # --- Environment resolution ---

  def test_env_from_explicit_option
    config = Wdk::Config.new(home_dir: @home, env_option: "staging")
    assert_equal "staging", config.environment
  end

  def test_env_from_WDK_ENV
    with_env("WDK_ENV" => "development", "KAMAL_DESTINATION" => nil) do
      config = Wdk::Config.new(home_dir: @home)
      assert_equal "development", config.environment
    end
  end

  def test_env_from_KAMAL_DESTINATION
    with_env("WDK_ENV" => nil, "KAMAL_DESTINATION" => "production") do
      config = Wdk::Config.new(home_dir: @home)
      assert_equal "production", config.environment
    end
  end

  def test_explicit_env_beats_WDK_ENV
    with_env("WDK_ENV" => "staging") do
      config = Wdk::Config.new(home_dir: @home, env_option: "production")
      assert_equal "production", config.environment
    end
  end

  def test_WDK_ENV_beats_KAMAL_DESTINATION
    with_env("WDK_ENV" => "staging", "KAMAL_DESTINATION" => "production") do
      config = Wdk::Config.new(home_dir: @home)
      assert_equal "staging", config.environment
    end
  end

  def test_raises_when_no_environment_source
    with_env("WDK_ENV" => nil, "KAMAL_DESTINATION" => nil) do
      config = Wdk::Config.new(home_dir: @home)
      assert_raises(Wdk::Config::MissingEnvironmentError) { config.environment! }
    end
  end

  # --- Secrets list from wdk.yml ---

  def test_loads_secrets_list_from_wdk_yml
    write_project_config(@project_dir, project: "my-app", secrets: %w[POSTGRES_PASSWORD STRIPE_KEY])
    config = Wdk::Config.new(home_dir: @home, start_dir: @project_dir)
    assert_equal %w[POSTGRES_PASSWORD STRIPE_KEY], config.secrets
  end

  def test_secrets_defaults_to_empty_array
    write_project_config(@project_dir, project: "my-app")
    config = Wdk::Config.new(home_dir: @home, start_dir: @project_dir)
    assert_equal [], config.secrets
  end

  private

  def write_user_config(token: nil, api_url: nil)
    dir = File.join(@home, ".wedokeys")
    FileUtils.mkdir_p(dir)
    path = File.join(dir, "config.yml")
    data = {}
    data["token"]   = token   if token
    data["api_url"] = api_url if api_url
    require "yaml"
    File.write(path, YAML.dump(data))
    File.chmod(0o600, path)
  end

  def write_project_config(dir, project:, secrets: nil)
    data = { "project" => project }
    data["secrets"] = secrets if secrets
    require "yaml"
    File.write(File.join(dir, "wdk.yml"), YAML.dump(data))
  end

  def with_env(vars, &block)
    old = vars.keys.each_with_object({}) { |k, h| h[k] = ENV[k] }
    vars.each { |k, v| v.nil? ? ENV.delete(k) : ENV[k] = v }
    block.call
  ensure
    old.each { |k, v| v.nil? ? ENV.delete(k) : ENV[k] = v }
  end
end
