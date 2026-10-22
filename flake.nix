{
  description = "OpenTofu/Terraform provider for Google Workspace";

  inputs = {
    # Own pin: go.mod needs Go >= 1.27.3, newer than some consumers' nixpkgs.
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      # Keep in step with the release tag; consumers pin this exact version.
      version = "0.1.3";

      systems = [ "x86_64-linux" "aarch64-linux" "x86_64-darwin" "aarch64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});

      # Every platform tofu may run on, so one mirror yields lock-file hashes for all.
      targets = [ "darwin_amd64" "darwin_arm64" "linux_amd64" "linux_arm64" ];

      ldflags = [ "-s" "-w" "-X main.version=${version}" ];
    in
    {
      packages = forAllSystems (pkgs: rec {
        # Match go.mod's toolchain; nixpkgs' default `go` lags a minor release.
        default = (pkgs.buildGoModule.override { go = pkgs.go_1_27; }) {
          pname = "terraform-provider-googleworkspace";
          inherit version ldflags;
          src = self;
          vendorHash = "sha256-KGNWzlyH79YtnadqHyG1OGqifAWGxZ3QmdnVj5TDfBc=";
          subPackages = [ "." ];
          env.CGO_ENABLED = "0";
        };

        # Unpacked filesystem-mirror layout for `provider_installation`:
        #   registry.opentofu.org/spokane-mountaineers/googleworkspace/<ver>/<os_arch>/<binary>
        # Cross-compiled with CGO off and -trimpath so the bytes, and therefore the
        # h1: lock-file hashes, do not depend on the build host.
        tofu-mirror = default.overrideAttrs (old: {
          pname = "terraform-provider-googleworkspace-tofu-mirror";
          doCheck = false;
          buildPhase = ''
            runHook preBuild
            for target in ${toString targets}; do
              GOOS="''${target%_*}" GOARCH="''${target#*_}" go build \
                -trimpath -buildvcs=false \
                -ldflags "${toString ldflags}" \
                -o "build/$target/terraform-provider-googleworkspace_v${version}" .
            done
            runHook postBuild
          '';
          installPhase = ''
            runHook preInstall
            dest="$out/registry.opentofu.org/spokane-mountaineers/googleworkspace/${version}"
            mkdir -p "$dest"
            cp -r build/* "$dest/"
            runHook postInstall
          '';
        });
      });

      checks = forAllSystems (pkgs: {
        inherit (self.packages.${pkgs.system}) default tofu-mirror;
      });
    };
}
