# Changelog

## 0.2.0 (2026-03-30)

### Features

- add --no-cache flag, device persistence, NWK_addr resolution, and supported devices table ([18c0023](https://github.com/urmzd/zigbee-skill/commit/18c0023c9a89d27b910658b48de713cfaabdc96b))
- **cli**: add network reset subcommand and reduce daemon logging ([6e60e4c](https://github.com/urmzd/zigbee-skill/commit/6e60e4c317739f05eb749d4eeb9266a37716aa74))
- **zigbee**: add network reset and message delivery tracking ([4f03c44](https://github.com/urmzd/zigbee-skill/commit/4f03c4467c91186ba4ef66e6dbd5c8cf6a8bc6a7))
- **zigbee**: implement security initialization and endpoint registration ([3d5a4e3](https://github.com/urmzd/zigbee-skill/commit/3d5a4e3990f481c955858c66b663d2c8e7aa21e5))
- **config**: add name field to configuration ([e46a8ef](https://github.com/urmzd/zigbee-skill/commit/e46a8efbe4a2f53ad53e153ea9268664c0544932))
- **device**: add clear all devices interface method ([ea49f9c](https://github.com/urmzd/zigbee-skill/commit/ea49f9cb0391e1e3b7544c7885c974b5d3b87670))
- **cli**: add daemon mode support with background fork ([2bfd657](https://github.com/urmzd/zigbee-skill/commit/2bfd657173d2a4bf41623d175ddeb6f9c06067dc))
- **daemon**: implement background daemon with Unix socket server ([4fbf6c1](https://github.com/urmzd/zigbee-skill/commit/4fbf6c1f7d56a57129d0264183bba461a34dc37b))
- **cli**: add interactive discovery mode with --wait-for option ([d962ae2](https://github.com/urmzd/zigbee-skill/commit/d962ae2742181b218a13263bc3da940a2fdb78fe))
- **controller**: discover device clusters via ZDO Simple Descriptor ([82cc1b7](https://github.com/urmzd/zigbee-skill/commit/82cc1b7239777ffb1becc39090676115f74d7f63))
- **config**: add endpoint and clusters to device persistence ([7e896c6](https://github.com/urmzd/zigbee-skill/commit/7e896c650d5226a35110bc33d95fc5c18f2ff0da))
- **zigbee**: add cluster constants and ZDO support ([fde1826](https://github.com/urmzd/zigbee-skill/commit/fde18269a10b9bb1039fa488d8efa4325174acf8))
- **skill**: add Zigbee Skill definition for agents ([40edc85](https://github.com/urmzd/zigbee-skill/commit/40edc8572f374f8040a0a61c69a8f9e182ca23b8))
- **config**: implement YAML-based configuration system ([3d13316](https://github.com/urmzd/zigbee-skill/commit/3d133167bf18bfda4e397b777d028e73754b657b))
- **controller**: implement BDB 3.1 device initialization ([7a19683](https://github.com/urmzd/zigbee-skill/commit/7a196838a87acf993b8bc9919118c192de0478d2))
- **ezsp**: add extended frame format and new commands ([96d0d1f](https://github.com/urmzd/zigbee-skill/commit/96d0d1fffd87704b32637090e3a49503a6a09fc5))
- **zcl**: add keep-alive cluster and reporting support ([6561c01](https://github.com/urmzd/zigbee-skill/commit/6561c013ad49a9719041ccbbe299b89deb772b8b))

### Bug Fixes

- correct build path from cmd/cli to cmd/zigbee-skill ([b7fd468](https://github.com/urmzd/zigbee-skill/commit/b7fd468fd6e8bbb6978983e11c46b65694f38e25))
- prevent unbounded log growth from ASH read error loop ([dc92fd3](https://github.com/urmzd/zigbee-skill/commit/dc92fd3adcd59725e6fa00d6270e163aca137a37))
- **lint**: resolve errcheck and unused code warnings ([f084b15](https://github.com/urmzd/zigbee-skill/commit/f084b150c5a6aaca867d0524c63ccfecf6636a27))
- **cli**: enable debug logging in daemon mode ([e4be068](https://github.com/urmzd/zigbee-skill/commit/e4be0682a5db4ec0b20b52eb4708d3e3b4f8ee94))
- **app**: skip device cache loading due to NodeID persistence issue ([73fe636](https://github.com/urmzd/zigbee-skill/commit/73fe63610f546d54c96e040f84a99addffea67dd))
- **ash**: implement data randomization per UG101 §4.3 ([3a17cb2](https://github.com/urmzd/zigbee-skill/commit/3a17cb2e5ccf423cca8e3618ddb9b66bdedc4c68))

### Documentation

- **faq**: add device pairing troubleshooting and clarify adapter selection ([a44096d](https://github.com/urmzd/zigbee-skill/commit/a44096dc50097a4dc8776fa885be4405f447b998))
- **readme**: update documentation for daemon-based architecture ([96ce936](https://github.com/urmzd/zigbee-skill/commit/96ce936aa2e18b62a84672052fbf184f2e0a4682))
- **AGENTS**: update specification references and architecture ([60bf800](https://github.com/urmzd/zigbee-skill/commit/60bf800c54e45818417326f6e0cc59deb9a676ce))
- add comprehensive zigbee protocol layer overview ([c9033ba](https://github.com/urmzd/zigbee-skill/commit/c9033baf2e5e342377c37ef14d56a63d6bc01fa0))
- update project documentation for AI-first approach ([0734543](https://github.com/urmzd/zigbee-skill/commit/07345434bb456456959536a1c52b8efea2f2d439))
- **zigbee**: add specifications and FAQ documentation ([59fff53](https://github.com/urmzd/zigbee-skill/commit/59fff53856ffc6ed34c65ffacbee958995194376))
- update README and teasr config ([4e02240](https://github.com/urmzd/zigbee-skill/commit/4e022404f95dd9f958bd7aa4840be501d74a234e))
- add showcase screenshots ([be229d5](https://github.com/urmzd/zigbee-skill/commit/be229d57cc4ddb92202a97e3bbbc3d7687d679ea))
- add showcase section to README ([0f401f5](https://github.com/urmzd/zigbee-skill/commit/0f401f58e5df5ff78b06578c3e919b122d4b3a2b))

### Refactoring

- **build**: reorganize command directory structure ([bcd7b40](https://github.com/urmzd/zigbee-skill/commit/bcd7b408572900a9f4c2f44481b4fa8044333efd))
- **app**: sync devices using exported model and fix IEEE parsing ([714f990](https://github.com/urmzd/zigbee-skill/commit/714f9907c8049c9a9439209f111c0a93ed920232))
- **cli**: migrate to YAML configuration ([68b7573](https://github.com/urmzd/zigbee-skill/commit/68b7573244a6650d6d8ad209256a854e2bb5c44f))
- **app**: integrate YAML configuration system ([a38802d](https://github.com/urmzd/zigbee-skill/commit/a38802d1e885084d10251f907e391c7dc43a611d))
- **db**: remove SQLite persistence layer ([c9df9a5](https://github.com/urmzd/zigbee-skill/commit/c9df9a591bac06a09260135b79985e38e4e858c3))
- **api**: remove REST API implementation ([da96975](https://github.com/urmzd/zigbee-skill/commit/da96975112fcc5eb88d9300475d85f280451f6c0))

### Miscellaneous

- update sr action from v2 to v3 ([f56ff99](https://github.com/urmzd/zigbee-skill/commit/f56ff9972759611f845fa2b7bc48a5e2d798a019))
- auto-format Go files with gofmt ([c011021](https://github.com/urmzd/zigbee-skill/commit/c011021ac053a574eebde212afe6deb226b5c5be))
- standardize CI/CD — refactor bump, workflow_dispatch, embed-src sync, justfile recipes ([831d9cf](https://github.com/urmzd/zigbee-skill/commit/831d9cff9b7890cc6aca61ca0ddf19a99916be87))
- remove unused sanity check script ([25fcf86](https://github.com/urmzd/zigbee-skill/commit/25fcf86ca86e98dd3337231208b94748a6613a00))
- **dist**: remove compiled cli binary ([409226a](https://github.com/urmzd/zigbee-skill/commit/409226a2ae65547bc287a79076d7b1736f352743))
- **cli**: remove old directory after reorganization ([9f77081](https://github.com/urmzd/zigbee-skill/commit/9f77081a306171530bea5a64a79e4234795379b2))
- **dist**: remove old api and zigbee-skill binaries, update cli ([d67ff56](https://github.com/urmzd/zigbee-skill/commit/d67ff56004e23a38b2e648ae5b5d3dbb132fc8ea))
- **config**: update serial port and device state ([6b7d6d1](https://github.com/urmzd/zigbee-skill/commit/6b7d6d102d28177394b9d22853c5df5038304291))
- remove mcp binary artifact ([ed63340](https://github.com/urmzd/zigbee-skill/commit/ed6334090d7a7d5c2e707c90c630cda3c01779ca))
- format zigbee protocol files with gofmt ([dd04753](https://github.com/urmzd/zigbee-skill/commit/dd047539504d66b395c838862a2815d82e820005))
- update device last seen timestamp ([5ee4d6e](https://github.com/urmzd/zigbee-skill/commit/5ee4d6e05b7fcaf17761ddb0d7cb691f3b6703aa))
- **dist**: add compiled binaries ([42b078f](https://github.com/urmzd/zigbee-skill/commit/42b078f51983ac15333423e26b9d26f0e0091bad))
- add test device to skill configuration ([3ad3920](https://github.com/urmzd/zigbee-skill/commit/3ad392093532e537cb3d653e2b6cc7338a31dfb5))
- **deps**: remove REST API dependencies ([fcce76f](https://github.com/urmzd/zigbee-skill/commit/fcce76ff8fdde55a6d3ee318a31eb2a224a4db03))
- **build**: remove REST API build targets ([d7eed37](https://github.com/urmzd/zigbee-skill/commit/d7eed3797239e4d95811690bf3cf24edeeb03d24))
- **ci**: add sr release hooks ([2e00f91](https://github.com/urmzd/zigbee-skill/commit/2e00f918a1823aecbf606a056c0a8eb956b1d0a1))
- **release**: configure sr with changelog sections ([cbafe92](https://github.com/urmzd/zigbee-skill/commit/cbafe92c4b6b8c9f1f786ecf9d72a08d1511c9f6))
- use sr-releaser GitHub App for release workflow (#7) ([5cde786](https://github.com/urmzd/zigbee-skill/commit/5cde7868027ec333f2f66f22ba64a5f38cc8ee4a))
- update semantic-release action to sr@v2 ([9f07d6a](https://github.com/urmzd/zigbee-skill/commit/9f07d6a37ded0aeac4890b6a1260e9943f8ab412))
- **deps**: bump urmzd/semantic-release from 1 to 2 ([bd26add](https://github.com/urmzd/zigbee-skill/commit/bd26add669efab67dc01cad3711be23cc36f45ef))
- rename homai references to zigbee-rest ([e0050eb](https://github.com/urmzd/zigbee-skill/commit/e0050eb3a80a8c23d71e8435d717737083106c42))

[Full Changelog](https://github.com/urmzd/zigbee-skill/compare/v0.1.0...v0.2.0)
