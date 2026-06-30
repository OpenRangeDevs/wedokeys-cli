require "test_helper"
require "wdk/commands/env"

class Wdk::Commands::EnvTest < Minitest::Test
  def setup
    @tmpdir = Dir.mktmpdir
    @home   = File.join(@tmpdir, "home")
    @project_dir = File.join(@tmpdir, "project")
    FileUtils.mkdir_p(File.join(@home, ".wedokeys"))
    FileUtils.mkdir_p(@project_dir)
    File.write(File.join(@home, ".wedokeys", "config.yml"),
      YAML.dump("token" => "wdk_sat_test", "api_url" => "https://app.wedokeys.com"))
    File.write(File.join(@project_dir, "wdk.yml"),
      YAML.dump("project" => "my-app", "secrets" => %w[STRIPE_KEY MISSING_KEY]))
  end

  def teardown
    FileUtils.rm_rf(@tmpdir)
  end

  def test_class_loads_without_error
    # Regression: "run" was a Thor reserved word and caused a RuntimeError at load time.
    assert_kind_of Class, Wdk::Commands::Env
  end

  def test_registers_exec_and_export_subcommands
    keys = Wdk::Commands::Env.commands.keys
    assert_includes keys, "exec"
    assert_includes keys, "export"
  end

  def test_does_not_register_run_as_subcommand
    refute_includes Wdk::Commands::Env.commands.keys, "run"
  end

  def test_exec_aborts_with_usage_when_no_args_given
    cmd = Wdk::Commands::Env.new
    err = assert_raises(SystemExit) { cmd.exec }
    assert_equal 1, err.status
  end

  def test_exec_aborts_when_some_aliases_unresolved
    stub_partial_resolve

    stderr = nil
    err = assert_raises(SystemExit) do
      stderr = capture_stderr do
        in_project { Wdk::Commands::Env.new.exec("true") }
      end
    end
    assert_equal 1, err.status
  end

  def test_exec_aborts_with_network_error_on_connection_refused
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve").to_raise(Errno::ECONNREFUSED)

    err = assert_raises(SystemExit) do
      capture_stderr { in_project { Wdk::Commands::Env.new.exec("true") } }
    end
    assert_equal 1, err.status
    assert_match "Network error", err.message
  end

  def test_export_aborts_when_some_aliases_unresolved
    stub_partial_resolve

    err = assert_raises(SystemExit) do
      capture_stderr { in_project { Wdk::Commands::Env.new.export } }
    end
    assert_equal 1, err.status
  end

  def test_export_prints_denials_to_stderr_before_aborting
    stub_partial_resolve

    stderr = StringIO.new
    old = $stderr
    $stderr = stderr
    assert_raises(SystemExit) { in_project { Wdk::Commands::Env.new.export } }
    assert_match "MISSING_KEY", stderr.string
    assert_match "Alias not found", stderr.string
  ensure
    $stderr = old
  end

  def test_export_with_allow_missing_prints_partial_results
    stub_partial_resolve

    out = capture_stdout do
      capture_stderr do
        in_project { Wdk::Commands::Env.new([], { "allow_missing" => true }).export }
      end
    end
    assert_match "export STRIPE_KEY=sk_live_abc", out
  end

  def test_export_aborts_cleanly_on_api_error
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 400,
        body: { error: "project_not_found", message: "Project not found" }.to_json,
        headers: { "Content-Type" => "application/json" })

    err = assert_raises(SystemExit) do
      capture_stderr { in_project { Wdk::Commands::Env.new.export } }
    end
    assert_equal 1, err.status
  end

  private

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

  def in_project(&block)
    old_home = ENV["HOME"]
    old_env = ENV["WDK_ENV"]
    ENV["HOME"] = @home
    ENV["WDK_ENV"] = "production"
    Dir.chdir(@project_dir, &block)
  ensure
    ENV["HOME"] = old_home
    old_env.nil? ? ENV.delete("WDK_ENV") : ENV["WDK_ENV"] = old_env
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
end
