# Arvados API server Docker container.

FROM arvados/passenger
MAINTAINER Ward Vandewege <ward@curoverse.com>

# Install postgres and apache.
RUN apt-get update -q
RUN apt-get install -qy \
    procps postgresql postgresql-server-dev-9.1 slurm-llnl munge \
    supervisor sudo libwww-perl libio-socket-ssl-perl libcrypt-ssleay-perl \
    libjson-perl cron openssh-server

ADD munge.key /etc/munge/
RUN chown munge:munge /etc/munge/munge.key && chmod 600 /etc/munge/munge.key
ADD generated/slurm.conf /etc/slurm-llnl/

RUN /usr/local/rvm/bin/rvm-exec default gem install arvados-cli arvados
# /for crunch-dispatch

RUN /bin/mkdir -p /usr/src/arvados/services
ADD generated/api.tar.gz /usr/src/arvados/services/

# Install generated config files
ADD generated/database.yml /usr/src/arvados/services/api/config/database.yml
ADD generated/omniauth.rb /usr/src/arvados/services/api/config/initializers/omniauth.rb
RUN /bin/cp /usr/src/arvados/services/api/config/environments/production.rb.example /usr/src/arvados/services/api/config/environments/production.rb
ADD generated/application.yml /usr/src/arvados/services/api/config/application.yml

# Configure Rails databases.
ENV RAILS_ENV production
ADD generated/config_databases.sh /tmp/config_databases.sh
ADD generated/superuser_token /tmp/superuser_token
RUN /usr/local/rvm/bin/rvm-exec default bundle install --gemfile=/usr/src/arvados/services/api/Gemfile && \
    sh /tmp/config_databases.sh && \
    rm /tmp/config_databases.sh && \
    /etc/init.d/postgresql start && \
    cd /usr/src/arvados/services/api && \
    /usr/local/rvm/bin/rvm-exec default bundle exec rake db:structure:load && \
    /usr/local/rvm/bin/rvm-exec default bundle exec rake db:seed && \
    /usr/local/rvm/bin/rvm-exec default bundle exec rake assets:precompile && \
    /usr/local/rvm/bin/rvm-exec default ./script/create_superuser_token.rb $(cat /tmp/superuser_token) && \
    chown www-data:www-data config.ru && \
    chown www-data:www-data log -R && \
    mkdir -p tmp && \
    chown www-data:www-data tmp -R

# Install a token for root
RUN mkdir -p /root/.config/arvados; echo "ARVADOS_API_HOST=api" >> /root/.config/arvados/settings.conf && echo "ARVADOS_API_HOST_INSECURE=yes" >> /root/.config/arvados/settings.conf && echo "ARVADOS_API_TOKEN=$(cat /tmp/superuser_token)" >> /root/.config/arvados/settings.conf && chmod 600 /root/.config/arvados/settings.conf

# Set up directory for job commit repo
RUN mkdir -p /var/lib/arvados
# Add crunch user
RUN addgroup --gid 4005 crunch && mkdir /home/crunch && useradd --uid 4005 --gid 4005 crunch && chown crunch:crunch /home/crunch

# Create keep and compute node objects
ADD generated/keep_server_0.json /root/
ADD generated/keep_server_1.json /root/
ADD keep_proxy.json /root/

# Set up update-gitolite.rb
RUN mkdir /usr/local/arvados/config -p
ADD generated/arvados-clients.yml /usr/src/arvados/services/api/config/
ADD .gitolite.rc /usr/local/arvados/config/
RUN ln /usr/src/arvados/services/api/script/arvados-git-sync.rb /usr/local/bin/

# Supervisor.
ADD supervisor.conf /etc/supervisor/conf.d/arvados.conf
ADD generated/setup.sh /usr/local/bin/setup.sh
ADD generated/setup-gitolite.sh /usr/local/bin/setup-gitolite.sh
ADD crunch-dispatch-run.sh /usr/local/bin/crunch-dispatch-run.sh
ADD munge.sh /usr/local/bin/munge.sh
ADD passenger.sh /usr/local/bin/passenger.sh

# Start the supervisor.
CMD ["/usr/bin/supervisord", "-n"]
