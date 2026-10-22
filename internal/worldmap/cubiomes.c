// Builds the vendored cubiomes sources as part of this package, since cgo
// only compiles C files that sit directly in the package directory.
#include "cubiomes/noise.c"
#include "cubiomes/biomes.c"
#include "cubiomes/biomenoise.c"
#include "cubiomes/layers.c"
#include "cubiomes/generator.c"
#include "cubiomes/finders.c"
#include "cubiomes/quadbase.c"
#include "cubiomes/util.c"
