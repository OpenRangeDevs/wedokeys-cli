require "test_helper"
require "wdk/client"

class Wdk::ClientTest < Minitest::Test
  def setup
    @client = Wdk::Client.new(
      api_url: "https://app.wedokeys.com",
      token: "wdk_sat_testtoken"
    )
  end

  def test_resolve_by_aliases_returns_resolved_hash
    stub_resolve(
      request_body: { aliases: %w[POSTGRES_PASSWORD], project: "my-app", environment: "production" },
      response_body: { resolved: { "POSTGRES_PASSWORD" => "secret123" }, errors: [], ttl_seconds: 300, request_id: "req_abc" }
    )

    result = @client.resolve_by_aliases(
      aliases: %w[POSTGRES_PASSWORD],
      project: "my-app",
      environment: "production"
    )

    assert_equal({ "POSTGRES_PASSWORD" => "secret123" }, result[:resolved])
    assert_empty result[:errors]
  end

  def test_resolve_by_aliases_raises_on_401
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 401, body: '{"error":"unauthorized"}', headers: { "Content-Type" => "application/json" })

    assert_raises(Wdk::Client::AuthError) do
      @client.resolve_by_aliases(aliases: %w[KEY], project: "app", environment: "production")
    end
  end

  def test_resolve_by_aliases_raises_on_400
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 400, body: '{"error":"missing_project","message":"project is required"}',
                 headers: { "Content-Type" => "application/json" })

    assert_raises(Wdk::Client::ApiError) do
      @client.resolve_by_aliases(aliases: %w[KEY], project: "", environment: "production")
    end
  end

  def test_api_error_carries_http_status
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 400, body: '{"error":"missing_project","message":"project is required"}',
                 headers: { "Content-Type" => "application/json" })

    error = assert_raises(Wdk::Client::ApiError) do
      @client.resolve_by_aliases(aliases: %w[KEY], project: "", environment: "production")
    end
    assert_equal 400, error.status
  end

  def test_api_error_status_for_server_error
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .to_return(status: 500, body: "", headers: {})

    error = assert_raises(Wdk::Client::ApiError) do
      @client.resolve_by_aliases(aliases: %w[KEY], project: "app", environment: "production")
    end
    assert_equal 500, error.status
  end

  def test_raises_network_error_on_connection_refused
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve").to_raise(Errno::ECONNREFUSED)

    assert_raises(Wdk::Client::NetworkError) do
      @client.resolve_by_aliases(aliases: %w[KEY], project: "app", environment: "production")
    end
  end

  def test_raises_network_error_on_timeout
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve").to_timeout

    assert_raises(Wdk::Client::NetworkError) do
      @client.resolve_by_aliases(aliases: %w[KEY], project: "app", environment: "production")
    end
  end

  def test_sends_correct_authorization_header
    stub = stub_resolve(
      request_body: { aliases: %w[KEY], project: "app", environment: "development" },
      response_body: { resolved: { "KEY" => "val" }, errors: [], ttl_seconds: 300, request_id: "req_x" }
    )

    @client.resolve_by_aliases(aliases: %w[KEY], project: "app", environment: "development")

    assert_requested stub
  end

  def test_sends_wdk_user_agent
    stub_resolve(
      request_body: { aliases: %w[KEY], project: "app", environment: "development" },
      response_body: { resolved: {}, errors: [], ttl_seconds: 300, request_id: "req_x" }
    )

    # If the User-Agent header is absent or wrong, webmock won't match the stub
    # and will raise a connection error — that's the implicit assertion.
    result = @client.resolve_by_aliases(aliases: %w[KEY], project: "app", environment: "development")
    assert_equal({}, result[:resolved])
  end

  private

  def stub_resolve(request_body:, response_body:)
    stub_request(:post, "https://app.wedokeys.com/api/v1/resolve")
      .with(
        body: request_body.to_json,
        headers: {
          "Authorization" => "Bearer wdk_sat_testtoken",
          "Content-Type"  => "application/json"
        }
      )
      .to_return(
        status: 200,
        body: response_body.to_json,
        headers: { "Content-Type" => "application/json" }
      )
  end
end
