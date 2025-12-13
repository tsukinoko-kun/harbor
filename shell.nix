{ pkgs ? import <nixpkgs> {} }:

let
  # Import nixGL so we can use it in the shell
  nixGL = import (builtins.fetchTarball "https://github.com/guibou/nixGL/archive/main.tar.gz") {};
in
pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    pkg-config
  ];

  buildInputs = with pkgs; [
    go
    vulkan-headers
    vulkan-loader
    wayland
    libxkbcommon
    libGL
    libffi
    xorg.libX11
    xorg.libXcursor
    xorg.libXfixes
    xorg.libxcb
    
    # Add nixGL to the environment
    nixGL.auto.nixGLDefault
  ];

  shellHook = ''
    export LD_LIBRARY_PATH=${pkgs.lib.makeLibraryPath [
      pkgs.libxkbcommon
      pkgs.libGL
      pkgs.vulkan-loader
      pkgs.wayland
      pkgs.xorg.libX11
      pkgs.xorg.libXcursor
      pkgs.xorg.libXfixes
      pkgs.xorg.libxcb
    ]}
    
    # Optional: Alias 'go run' to always use nixGL inside this shell
    alias gorun="nixGL go run"
    
    echo "Environment loaded."
    echo "Tip: Run 'nixGL go run .' (or the 'gorun' alias) to start your app."
  '';
}