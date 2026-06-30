require "test_helper"
require "wdk/commands/login"

class Wdk::Commands::LoginTest < Minitest::Test
  def setup
    @tmpdir = Dir.mktmpdir
    @home   = File.join(@tmpdir, "home")
    FileUtils.mkdir_p(@home)
  end

  def teardown
    FileUtils.rm_rf(@tmpdir)
  end

  def test_saves_token_when_probe_returns_400
    stub_verify(status: 400, body: { error: "missing_project" })

    Wdk::Commands::Login.new({ "token" => "wdk_sat_valid" }, home_dir: @home).run

    assert_equal "wdk_sat_valid", saved_token
  end

  def test_aborts_without_saving_on_server_error
    stub_verify(status: 500, body: {})

    err = assert_raises(SystemExit) do
      capture_io { Wdk::Commands::Login.new({ "token" => "wdk_sat_x" }, home_dir: @home).run }
    end
    assert_equal 1, err.status
    assert_nil saved_token
  end

  def test_aborts_without_saving_on_invalid_token
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 401, body: '{"error":"unauthorized"}', headers: { "Content-Type" => "application/json" })

    assert_raises(SystemExit) do
      capture_io { Wdk::Commands::Login.new({ "token" => "wdk_sat_bad" }, home_dir: @home).run }
    end
    assert_nil saved_token
  end

  def test_aborts_without_saving_on_network_error
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve").to_raise(Errno::ECONNREFUSED)

    assert_raises(SystemExit) do
      capture_io { Wdk::Commands::Login.new({ "token" => "wdk_sat_x" }, home_dir: @home).run }
    end
    assert_nil saved_token
  end

  private

  def stub_verify(status:, body:)
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: status, body: body.to_json, headers: { "Content-Type" => "application/json" })
  end

  def saved_token
    path = File.join(@home, ".wedokeys", "config.yml")
    return nil unless File.exist?(path)
    YAML.safe_load_file(path)["token"]
  end
end
