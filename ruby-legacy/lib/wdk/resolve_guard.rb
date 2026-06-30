module Wdk
  # Shared handling for per-alias resolution failures. Secrets fetching is
  # all-or-nothing by default: proceeding with partial results means booting
  # an app with missing env vars, so any unresolved alias aborts unless the
  # caller passed --allow-missing.
  module ResolveGuard
    module_function

    def check!(result, allow_missing: false)
      errors = result[:errors]
      return result if errors.empty?

      errors.each do |err|
        $stderr.puts "#{err["reference"]}: #{err["message"]} (#{err["code"]})"
      end
      return result if allow_missing

      total = result[:resolved].size + errors.size
      abort "Error: #{errors.size} of #{total} secrets could not be resolved. " \
            "Fix the aliases above, or pass --allow-missing to proceed without them."
    end
  end
end
