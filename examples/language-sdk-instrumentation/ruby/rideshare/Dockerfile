FROM ruby:3.3.6

WORKDIR /opt/app

COPY Gemfile ./Gemfile
COPY Gemfile.lock ./Gemfile.lock
# RUN bundle config set --local deployment true
RUN bundle install

COPY lib ./lib

CMD [ "ruby", "lib/server.rb" ]
