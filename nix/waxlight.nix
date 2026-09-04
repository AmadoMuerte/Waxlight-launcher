{
  lib,
  pkgs,
  go,
  nodejs_22,
  pkg-config,
  gtk3,
  webkitgtk_4_1,
  glib-networking,
  gsettings-desktop-schemas,
  shared-mime-info,
  wrapGAppsHook3,
}:
let
  # The launcher version is read from the same wails.json that is embedded
  # into the binary, so a Nix build reports the version of the source tree.
  version = (builtins.fromJSON (builtins.readFile ../wails.json)).info.productVersion;

  # The derivation builds a production GUI binary. Local build artifacts are
  # excluded so the store source is reproducible and does not carry leftovers.
  src =
    let
      root = toString ../.;
      denied = [
        ".git/"
        "frontend/node_modules/"
        "frontend/dist/"
        "docs/generated/"
        "docs/site/node_modules/"
        "release/"
        "build/bin/"
        "result"
        "result-"
      ];
      filter = path: type:
        let
          relative = lib.removePrefix (root + "/") (toString path);
        in
        !(lib.any (prefix: lib.hasPrefix prefix relative) denied);
    in
    lib.cleanSourceWith {
      src = ../.;
      inherit filter;
    };

  # npm dependencies are rebuilt from package-lock.json into an offline,
  # store-based node_modules. The check:i18n step and the production bundle
  # run against the repository layout inside the same build. Local install
  # leftovers are filtered out like in the main source.
  frontendSrc = lib.cleanSourceWith {
    src = ../frontend;
    filter = path: type:
      let
        base = baseNameOf (toString path);
      in
      !(lib.elem base [ "node_modules" "dist" ".vite" ]);
  };
  nodeModules = pkgs.importNpmLock.buildNodeModules {
    npmRoot = frontendSrc;
    nodejs = nodejs_22;
  };

  # Vintage Story on Linux is a .NET application. The launcher detects a
  # compatible runtime and injects DOTNET_ROOT/PATH into the game process, so
  # the package ships the required runtime instead of relying on the system.
  dotnetRoot = "${pkgs.dotnetCorePackages.dotnet_10.runtime.unwrapped}/share/dotnet";

  # The game is a third-party binary (not built with Nix) and needs the system
  # C++ runtime, the bundled glfw's X11/OpenGL stack, and the native libraries
  # it P/Invokes at startup (Cairo and friends), so the launcher's wrapper
  # exposes them through LD_LIBRARY_PATH.
  gameLibs = with pkgs; [
    gcc.cc.lib
    libGL
    libGLU
    libX11
    libXcursor
    libXi
    libXrandr
    libXxf86vm
    libxkbcommon
    libxrender
    libxext
    libxau
    libxdmcp
    libxcb
    fontconfig
    cairo
    freetype
    pixman
    libpng
    expat
    brotli
    bzip2
    zlib
  ];

  # The NixOS stub loader at /lib64/ld-linux-x86-64.so.2 refuses to run generic
  # dynamic executables, so the launcher is pointed at the packaged glibc
  # loader and rewrites the game's ELF interpreter to it (with patchelf).
  elfInterpreter = "${pkgs.glibc.out}/lib/ld-linux-x86-64.so.2";
in
# buildGoModule provides the Go module cache through vendorHash (a fixed-output
# derivation), so the build is fully reproducible inside the Nix sandbox. The
# build and install phases are replaced with Waxlight's own production build.
pkgs.buildGoModule {
  pname = "waxlight";
  inherit version src;

  nativeBuildInputs = [
    wrapGAppsHook3
    pkg-config
    go
    nodejs_22
  ];

  buildInputs = [
    gtk3
    webkitgtk_4_1
    glib-networking
    gsettings-desktop-schemas
    shared-mime-info
  ];

  # Go modules are fetched and vendored from the prefetched store directory.
  vendorHash = "sha256-luTPr+rMlR/hZxqBNm5/O1Yk6Cd8dzsPJlls31TvfAM=";

  buildPhase = ''
    runHook preBuild
    cp -a ${frontendSrc}/. frontend/
    # copy merges into the existing read-only store layout; keep it writable
    # so the frontend build can emit dist/ and default tooling caches.
    chmod -R u+w frontend
    ln -s ${nodeModules}/node_modules frontend/node_modules
    npm run build --prefix frontend
    go build \
      -buildvcs=false \
      -tags "desktop,production,webkit2_41" \
      -ldflags "-s -w -X github.com/AmadoMuerte/Waxlight-launcher/internal/version.buildVersion=${version} -X github.com/AmadoMuerte/Waxlight-launcher/internal/updates.externallyManaged=1" \
      -o waxlight \
      ./cmd/waxlight
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    install -Dm755 waxlight $out/bin/waxlight
    install -Dm644 packaging/linux/com.waxlight.launcher.desktop $out/share/applications/com.waxlight.launcher.desktop
    install -Dm644 packaging/linux/com.waxlight.launcher.svg $out/share/icons/hicolor/scalable/apps/com.waxlight.launcher.svg
    install -Dm644 LICENSE $out/share/licenses/waxlight/LICENSE
    install -Dm644 NOTICE $out/share/doc/waxlight/NOTICE
    install -Dm644 README.md $out/share/doc/waxlight/README.md
    runHook postInstall
  '';

  # Expose the bundled .NET runtime through DOTNET_ROOT and PATH so the
  # launcher finds it and passes it to the game process, expose the third-party
  # game's native library dependencies through LD_LIBRARY_PATH, and point the
  # launcher at the packaged ELF loader plus patchelf for the game binary.
  preFixup = let
    libraryPath = lib.makeLibraryPath gameLibs;
  in ''
    gappsWrapperArgs+=(
      --set DOTNET_ROOT ${dotnetRoot}
      --set WAXLIGHT_ELF_INTERPRETER ${elfInterpreter}
      --prefix PATH : ${pkgs.patchelf}/bin
      --prefix PATH : ${dotnetRoot}
      --prefix LD_LIBRARY_PATH : ${libraryPath}
    )
  '';

  meta = {
    description = "A modern, lightweight launcher for Vintage Story";
    homepage = "https://github.com/AmadoMuerte/Waxlight-launcher";
    license = lib.licenses.gpl3Only;
    platforms = [ "x86_64-linux" ];
    mainProgram = "waxlight";
  };
}
