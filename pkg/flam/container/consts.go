package container

// BagPathSeparator is the string separating the segments of a [Bag] path.
// The path "a.b.c" addresses the value stored under key "c" of the bag
// stored under key "b" of the bag stored under key "a".
const BagPathSeparator = "."

// Context fields of the [*github.com/cjdias/flam-in-go/pkg/flam.Error]
// values returned by [Bag.Set], [Bag.Delete] and [Bag.Merge], as
// documented by each method.
const (
	// ErrorFieldPath holds the path given to the method.
	ErrorFieldPath = "path"

	// ErrorFieldSegment holds the path up to the segment that holds a
	// value that is not a bag.
	ErrorFieldSegment = "segment"

	// ErrorFieldKey holds the key of the bag merged by [Bag.Merge] that
	// is not a valid path. For a key of a nested bag, it holds the keys
	// leading to it from the merged bag, joined by [BagPathSeparator], as
	// in "a.b..c" for the key "b..c" of the bag under the key "a".
	ErrorFieldKey = "key"
)
