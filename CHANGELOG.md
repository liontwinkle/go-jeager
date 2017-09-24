Changes by Version
==================

0.8.0 (2017-09-24)
------------------

- Convert to Apache 2.0 License


0.7.0 (2017-08-22)
------------------

- Add health check server to collector and query [#280](https://github.com/uber/jaeger/pull/280)
- Add/fix sanitizer for Zipkin span start time and duration [#333](https://github.com/uber/jaeger/pull/333)
- Support Zipkin json encoding for /api/v1/spans HTTP endpoint [#348](https://github.com/uber/jaeger/pull/348)
- Support Zipkin 128bit traceId and ipv6 [#349](https://github.com/uber/jaeger/pull/349)


0.6.0 (2017-08-09)
------------------

- Add viper/cobra configuration support [#245](https://github.com/uber/jaeger/pull/245) [#307](https://github.com/uber/jaeger/pull/307)
- Add Zipkin /api/v1/spans endpoint [#282](https://github.com/uber/jaeger/pull/282)
- Add basic authenticator to configs for cassandra [#323](https://github.com/uber/jaeger/pull/323)
- Add Elasticsearch storage support


0.5.2 (2017-07-20)
------------------

- Use official Cassandra 3.11 base image [#278](https://github.com/uber/jaeger/pull/278)
- Support configurable metrics backend in the agent [#275](https://github.com/uber/jaeger/pull/275)


0.5.1 (2017-07-03)
------------------

- Bug fix: Query startup should fail when -query.static-files has no trailing slash [#144](https://github.com/uber/jaeger/issues/144)
- Push to Docker Hub on release tags [#246](https://github.com/uber/jaeger/pull/246)


0.5.0 (2017-07-01)
------------------

First numbered release.

