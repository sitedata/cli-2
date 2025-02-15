package lib

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

// CmdRange2CIDRFlags are flags expected by CmdRange2CIDR.
type CmdRange2CIDRFlags struct {
	Help bool
}

// Init initializes the common flags available to CmdRange2CIDR with sensible
// defaults.
//
// pflag.Parse() must be called to actually use the final flag values.
func (f *CmdRange2CIDRFlags) Init() {
	pflag.BoolVarP(
		&f.Help,
		"help", "h", false,
		"show help.",
	)
}

// CmdRange2CIDR is the common core logic for the range2cidr command.
func CmdRange2CIDR(
	f CmdRange2CIDRFlags,
	args []string,
	printHelp func(),
) error {
	if f.Help {
		printHelp()
		return nil
	}

	// require args.
	stat, _ := os.Stdin.Stat()
	isStdin := (stat.Mode() & os.ModeCharDevice) == 0
	if len(args) == 0 && !isStdin {
		printHelp()
		return nil
	}

	// actual scanner.
	scanrdr := func(r io.Reader) {
		var rem string
		var hitEOF bool
		var tmp string

		// will use this var temporarily to help us convert header.
		headerData := 0

		buf := bufio.NewReader(r)
		for {
			if hitEOF {
				return
			}

			d, err := buf.ReadString('\n')
			if err == io.EOF {
				if len(d) == 0 {
					return
				}

				// do one more loop on remaining content.
				hitEOF = true
			} else if err != nil {
				// TODO print error but have a `-q` flag to be quiet.
				return
			}

			sepIdx := strings.IndexAny(d, ",\n")
			if sepIdx == -1 {
				// only possible if EOF & input doesn't end with newline.
				sepIdx = len(d)
				rem = "\n"
			} else {
				// did we get an IP range with a comma delim?
				// if so, try again against the next delim.
				if sepIdx != len(d)-1 &&
					d[sepIdx] == ',' &&
					StrIsIPStr(d[:sepIdx]) {
					nextSepIdx := strings.IndexAny(d[sepIdx+1:], ",\n")
					if nextSepIdx == -1 {
						sepIdx = len(d)
						rem = "\n"
					} else {
						sepIdx = nextSepIdx + sepIdx + 1
						rem = d[sepIdx:]
					}
				} else {
					rem = d[sepIdx:]
				}
			}

			rangeStr := d[:sepIdx]
			if strings.IndexByte(rangeStr, ':') == -1 {
				if cidrs, err := CIDRsFromIPRangeStrRaw(rangeStr); err == nil {
					if headerData == 1 {
						headerData = 2

						fmt.Printf("cidr%s", tmp)
					}

					for _, cidr := range cidrs {
						fmt.Printf("%s%s", cidr, rem)
					}
				} else {
					goto noip
				}
			} else {
				if cidrs, err := CIDRsFromIP6RangeStrRaw(rangeStr); err == nil {
					if headerData == 1 {
						headerData = 2

						fmt.Printf("cidr%s", tmp)
					}

					for _, cidr := range cidrs {
						fmt.Printf("%s%s", cidr, rem)
					}
				} else {
					goto noip
				}
			}

			continue

		noip:
			if headerData == 0 {
				headerData = 1

				// temporarily buffer the remaining line, which is the part of
				// the header that we still care about.
				//
				// in the next iter, we'll be able to determine whether the
				// range input is `-` or `,` separated, which then tells us
				// what to print as the prefix.
				tmp = rem
			} else {
				fmt.Printf("%s", d)
			}
			if sepIdx == len(d) {
				fmt.Println()
			}
		}
	}

	// scan stdin first.
	if isStdin {
		scanrdr(os.Stdin)
	}

	// scan all args.
	for _, arg := range args {
		f, err := os.Open(arg)
		if err != nil {
			// is it an IP range?
			if strings.IndexByte(arg, ':') == -1 {
				if cidrs, err := CIDRsFromIPRangeStrRaw(arg); err == nil {
					for _, cidr := range cidrs {
						fmt.Println(cidr)
					}
					continue
				}
			} else {
				if cidrs, err := CIDRsFromIP6RangeStrRaw(arg); err == nil {
					for _, cidr := range cidrs {
						fmt.Println(cidr)
					}
					continue
				}
			}

			// invalid file arg.
			return err
		}

		scanrdr(f)
	}

	return nil
}
