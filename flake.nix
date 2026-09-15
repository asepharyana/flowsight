{
  description = "FlowSight — Sectors hackathon screener (Go chi backend + SolidJS web, single service)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachSystem [ "x86_64-linux" ] (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          config.allowUnfree = true;
        };

        version = "0.1.0";

        # pnpm install runs in CI sandbox with network; no prefetch pinning needed.
        flowsight = pkgs.stdenvNoCC.mkDerivation {
          pname = "flowsight";
          inherit version;
          src = ./.;

          nativeBuildInputs = with pkgs; [
            cacert curl gcc gnumake openssl pkg-config
            go_1_25 nodejs_22 corepack_22
          ];

          SSL_CERT_FILE = "${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt";
          NIX_ENFORCE_PURITY = "0";
          COREPACK_ENABLE_DOWNLOAD_PROMPT = "0";

          phases = [ "unpackPhase" "buildPhase" "installPhase" ];

          buildPhase = ''
            export HOME="$TMPDIR" GOCACHE="$TMPDIR/gocache" GOMODCACHE="$TMPDIR/gomodcache"
            export COREPACK_HOME="$TMPDIR/corepack" PNPM_HOME="$TMPDIR/pnpm-home"
            export PATH="$PNPM_HOME:$PATH"

            echo "=== Building backend ==="
            cd backend
            go build -trimpath -o $TMPDIR/flowsight ./cmd/server
            cd ..

            echo "=== Building web ==="
            cd web
            corepack pnpm install --frozen-lockfile --offline 2>/dev/null \
              || corepack pnpm install --frozen-lockfile
            corepack pnpm build
            cd ..

            echo "=== Bundle fixtures (offline seed at first boot) ==="
            mkdir -p $TMPDIR/fixtures
            cp backend/tests/fixtures/*.json $TMPDIR/fixtures/
          '';

          installPhase = ''
            mkdir -p $out/bin $out/share/flowsight-web $out/share/flowsight
            cp $TMPDIR/flowsight $out/bin/flowsight
            cp -r web/dist $out/share/flowsight-web/dist
            cp $TMPDIR/fixtures/*.json $out/share/flowsight/
          '';
        };
      in
      {
        packages = {
          inherit flowsight;
          default = flowsight;
        };

        apps.flowsight = {
          type = "app";
          program = "${flowsight}/bin/flowsight";
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go_1_25 nodejs_22 corepack_22 ];
        };
      });
}
