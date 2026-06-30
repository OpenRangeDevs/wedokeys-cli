require "test_helper"
require "wdk/config"
require "wdk/client"
require "wdk/commands/kamal_fetch"

class Wdk::Commands::KamalFetchTest < Minitest::Test
  def setup
    @tmpdir = Dir.mktmpdir
    @home   = File.join(@tmpdir, "home")
    @project_dir = File.join(@tmpdir, "project")
    FileUtils.mkdir_p(@home)
    FileUtils.mkdir_p(@project_dir)
    write_user_config
    write_project_config
  end

  def teardown
    FileUtils.rm_rf(@tmpdir)
  end

  def test_outputs_name_equals_value_lines
    stub_resolve(%w[POSTGRES_PASSWORD STRIPE_KEY],
      "POSTGRES_PASSWORD" => "pg_secret", "STRIPE_KEY" => "sk_live_abc")

    out = capture_stdout do
      with_env("KAMAL_DESTINATION" => "production") do
        cmd = Wdk::Commands::KamalFetch.new(
          { "from" => nil },
          %w[POSTGRES_PASSWORD STRIPE_KEY],
          home_dir: @home, start_dir: @project_dir
        )
        cmd.run
      end
    end

    assert_match "POSTGRES_PASSWORD=pg_secret", out
    assert_match "STRIPE_KEY=sk_live_abc", out
  end

  def test_parses_project_and_env_from_from_option
    stub_resolve(%w[DB_URL], "DB_URL" => "postgres://localhost")

    out = capture_stdout do
      cmd = Wdk::Commands::KamalFetch.new(
        { "from" => "other-app/staging" },
        %w[DB_URL],
        home_dir: @home, start_dir: @tmpdir
      )
      cmd.run
    end

    assert_match "DB_URL=postgres://localhost", out
  end

  def test_aborts_when_some_aliases_unresolved
    stub_partial_resolve

    cmd = Wdk::Commands::KamalFetch.new(
      { "from" => nil },
      %w[STRIPE_KEY MISSING_KEY],
      home_dir: @home, start_dir: @project_dir
    )

    err = nil
    stderr = capture_stderr do
      with_env("KAMAL_DESTINATION" => "production") do
        err = assert_raises(SystemExit) { cmd.run }
      end
    end

    assert_equal 1, err.status
    assert_match "MISSING_KEY", stderr
  end

  def test_allow_missing_proceeds_with_partial_results
    stub_partial_resolve

    cmd = Wdk::Commands::KamalFetch.new(
      { "from" => nil, "allow_missing" => true },
      %w[STRIPE_KEY MISSING_KEY],
      home_dir: @home, start_dir: @project_dir
    )

    out = capture_stdout do
      capture_stderr do
        with_env("KAMAL_DESTINATION" => "production") { cmd.run }
      end
    end

    assert_match "STRIPE_KEY=sk_live_abc", out
  end

  def test_aborts_when_value_contains_newline
    stub_resolve(%w[TLS_KEY], "TLS_KEY" => "-----BEGIN KEY-----\nabc\n-----END KEY-----")

    cmd = Wdk::Commands::KamalFetch.new(
      { "from" => nil }, %w[TLS_KEY],
      home_dir: @home, start_dir: @project_dir
    )

    out = nil
    err = nil
    stderr = capture_stderr do
      out = capture_stdout do
        with_env("KAMAL_DESTINATION" => "production") do
          err = assert_raises(SystemExit) { cmd.run }
        end
      end
    end

    assert_equal 1, err.status
    assert_match "TLS_KEY", stderr
    refute_match(/BEGIN KEY/, out) # never emit a partial/corrupt line
  end

  def test_aborts_when_value_is_a_hash_whole_secret
    stub_resolve(%w[DB], "DB" => { "user" => "u", "password" => "p" })

    cmd = Wdk::Commands::KamalFetch.new(
      { "from" => nil }, %w[DB],
      home_dir: @home, start_dir: @project_dir
    )

    err = nil
    stderr = capture_stderr do
      capture_stdout do
        with_env("KAMAL_DESTINATION" => "production") do
          err = assert_raises(SystemExit) { cmd.run }
        end
      end
    end

    assert_equal 1, err.status
    assert_match "DB", stderr
  end

  def test_exits_with_error_when_no_aliases_given
    cmd = Wdk::Commands::KamalFetch.new(
      { "from" => nil }, [],
      home_dir: @home, start_dir: @project_dir
    )
    err = assert_raises(SystemExit) { cmd.run }
    assert_equal 1, err.status
  end

  private

  def write_user_config
    dir = File.join(@home, ".wedokeys")
    FileUtils.mkdir_p(dir)
    path = File.join(dir, "config.yml")
    File.write(path, YAML.dump("token" => "wdk_sat_test", "api_url" => "https://app.wedokeys.com"))
    File.chmod(0o600, path)
  end

  def write_project_config
    File.write(File.join(@project_dir, "wdk.yml"), YAML.dump("project" => "my-app"))
  end

  def stub_resolve(aliases, resolved)
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 200, body: { resolved: resolved, errors: [], ttl_seconds: 300, request_id: "req_x" }.to_json,
                 headers: { "Content-Type" => "application/json" })
  end

  def stub_partial_resolve
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 200,
        body: {
          resolved: { "STRIPE_KEY" => "sk_live_abc" },
          errors: [ { "reference" => "MISSING_KEY", "code" => "not_found", "message" => "Alias not found" } ],
          ttl_seconds: 300, request_id: "req_x"
        }.to_json,
        headers: { "Content-Type" => "application/json" })
  end

  def capture_stdout
    old = $stdout
    $stdout = StringIO.new
    yield
    $stdout.string
  ensure
    $stdout = old
  end

  def capture_stderr
    old = $stderr
    $stderr = StringIO.new
    yield
    $stderr.string
  ensure
    $stderr = old
  end

  def with_env(vars)
    old = vars.keys.each_with_object({}) { |k, h| h[k] = ENV[k] }
    vars.each { |k, v| v.nil? ? ENV.delete(k) : ENV[k] = v }
    yield
  ensure
    old.each { |k, v| v.nil? ? ENV.delete(k) : ENV[k] = v }
  end
end
