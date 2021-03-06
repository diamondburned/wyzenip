{ unstable ? import <unstable> {} }:

unstable.stdenv.mkDerivation rec {
	name = "wyzenip";
	version = "0.0.1";

	buildInputs = with unstable; [
		gnome3.glib gnome3.gtk libhandy fftw portaudio
	];

	nativeBuildInputs = with unstable; [
		pkgconfig go
	];
}
