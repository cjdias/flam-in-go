// Package env is the configuration source driver of the environment
// variables: its sources map environment variables to configuration paths,
// as a [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource].
//
// A source is configured with:
//   - [ConfigFieldMappings]: a bag mapping environment variable names to
//     dot separated configuration paths, such as
//     {"APP_PORT": "server.port"};
//   - [ConfigFieldFiles]: the dotenv files, such as ".env", read for the
//     variables missing from the process environment, a file overriding
//     the files listed before it;
//   - [ConfigFieldPriority]: the priority of the source;
//   - [ConfigFieldReload]: whether the source is reloaded, true by default.
//     When false, the source is created as a
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigSource] only, not
//     implementing
//     [github.com/cjdias/flam-in-go/pkg/flam.IConfigReloadableSource], so
//     that the reloader of the configuration skips it and its values never
//     change.
//
// A missing field takes the value stored at [DefaultMappingsConfigPath],
// [DefaultFilesConfigPath], [DefaultPriorityConfigPath] or
// [DefaultReloadConfigPath], set by the bundle to no mapping, no file, 0
// and true. The files are read, not loaded: the
// process environment is never changed.
//
// A variable that is not set, or set to the empty string, leaves its path
// out of the source. The values of the others become:
//   - true and false, in any case: a bool;
//   - integers written as [strconv.Itoa] writes them, such as 8080 or -1:
//     an int, so that the int getters of the configuration read them;
//   - finite decimals written as the shortest [strconv.FormatFloat] does,
//     such as 0.5: a float64;
//   - anything else, such as 0755, 1.20, +1, NaN, Inf or yes: the string
//     itself.
//
// Reload, when activated, reads the files and the environment again.
package env
