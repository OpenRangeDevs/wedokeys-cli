require "net/http"
require "json"
require "uri"
require_relative "version"

module Wdk
  class Client
    class AuthError < StandardError; end

    class ApiError < StandardError
      attr_reader :status

      def initialize(message, status: nil)
        super(message)
        @status = status
      end
    end

    # Raised when the API can't be reached at all (DNS, refused, timeout, TLS).
    class NetworkError < StandardError; end

    USER_AGENT = "wedokeys-wdk-cli/#{Wdk::VERSION}"
    OPEN_TIMEOUT = 10 # seconds to establish the connection
    READ_TIMEOUT = 15 # seconds to wait for the response

    def initialize(api_url:, token:)
      @api_url = api_url.chomp("/")
      @token   = token
    end

    def resolve_by_aliases(aliases:, project:, environment:)
      body = { aliases: aliases, project: project, environment: environment }
      post("/api/v1/resolve", body)
    end

    private

    def post(path, body)
      uri  = URI("#{@api_url}#{path}")
      http = Net::HTTP.new(uri.host, uri.port)
      http.use_ssl = uri.scheme == "https"
      http.open_timeout = OPEN_TIMEOUT
      http.read_timeout = READ_TIMEOUT

      request = Net::HTTP::Post.new(uri.path)
      request["Authorization"] = "Bearer #{@token}"
      request["Content-Type"]  = "application/json"
      request["Accept"]        = "application/json"
      request["User-Agent"]    = USER_AGENT
      request.body             = body.to_json

      response = http.request(request)

      case response.code.to_i
      when 200..299
        parsed = JSON.parse(response.body)
        { resolved: parsed["resolved"] || {}, errors: parsed["errors"] || [] }
      when 401
        raise AuthError, "Authentication failed. Run `wdk login` to refresh your token."
      else
        parsed = JSON.parse(response.body) rescue {}
        raise ApiError.new(parsed["message"] || "API error #{response.code}", status: response.code.to_i)
      end
    rescue Errno::ECONNREFUSED, SocketError, Net::OpenTimeout, Net::ReadTimeout, OpenSSL::SSL::SSLError => e
      raise NetworkError, "Could not reach #{@api_url}: #{e.message}"
    end
  end
end
