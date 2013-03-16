Server::Application.configure do
  # Settings specified here will take precedence over those in config/application.rb

  # The test environment is used exclusively to run your application's
  # test suite.  You never need to work with it otherwise.  Remember that
  # your test database is "scratch space" for the test suite and is wiped
  # and recreated between test runs.  Don't rely on the data there!
  config.cache_classes = true

  # Configure static asset server for tests with Cache-Control for performance
  config.serve_static_assets = true
  config.static_cache_control = "public, max-age=3600"

  # Log error messages when you accidentally call methods on nil
  config.whiny_nils = true

  # Show full error reports and disable caching
  config.consider_all_requests_local       = true
  config.action_controller.perform_caching = false

  # Raise exceptions instead of rendering exception templates
  config.action_dispatch.show_exceptions = false

  # Disable request forgery protection in test environment
  config.action_controller.allow_forgery_protection    = false

  # Tell Action Mailer not to deliver emails to the real world.
  # The :test delivery method accumulates sent emails in the
  # ActionMailer::Base.deliveries array.
  config.action_mailer.delivery_method = :test

  # Use SQL instead of Active Record's schema dumper when creating the test database.
  # This is necessary if your schema can't be completely dumped by the schema dumper,
  # like if you have constraints or database-specific column types
  # config.active_record.schema_format = :sql

  # Print deprecation notices to the stderr
  config.active_support.deprecation = :stderr

  config.whjobmanager_wrapper = :slurm_immediate

  # config.dnsmasq_conf_dir = '/etc/dnsmasq.d'

  # config.compute_node_ami = 'ami-cbca41a2'
  # config.compute_node_security_group = 'orvos-compute'
  # config.compute_node_spot_bid = 0.11

  # config.compute_node_domain = `hostname --domain`.strip

  # No need for SSL while testing
  config.force_ssl = false

  # config.compute_node_nameservers = ['1.2.3.4', '1.2.3.5']
  config.compute_node_nameservers = [ "172.16.0.23" ]

  config.uuid_prefix('test@' + `hostname`.strip)

  # Authentication stub: hard code pre-approved API tokens.
  # config.accept_api_token = { rand(2**256).to_s(36) => true }
  config.accept_api_token = {}
end
