// Code for generating pull lists as described in https://arvados.org/projects/arvados/wiki/Keep_Design_Doc#Pull-List

package summary

import (
	"encoding/json"
	"fmt"
	"git.curoverse.com/arvados.git/sdk/go/blockdigest"
	"git.curoverse.com/arvados.git/sdk/go/keepclient"
	"git.curoverse.com/arvados.git/sdk/go/logger"
	"git.curoverse.com/arvados.git/services/datamanager/keep"
	"log"
	"os"
	"strings"
)

// Locator is a block digest
type Locator blockdigest.DigestWithSize

// MarshalJSON encoding
func (l Locator) MarshalJSON() ([]byte, error) {
	return []byte("\"" + blockdigest.DigestWithSize(l).String() + "\""), nil
}

// PullRequest represents one entry in the Pull List
type PullRequest struct {
	Locator Locator  `json:"locator"`
	Servers []string `json:"servers"`
}

// PullList for a particular server
type PullList []PullRequest

// PullListByLocator implements sort.Interface for PullList based on
// the Digest.
type PullListByLocator PullList

func (a PullListByLocator) Len() int      { return len(a) }
func (a PullListByLocator) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a PullListByLocator) Less(i, j int) bool {
	di, dj := a[i].Locator.Digest, a[j].Locator.Digest
	if di.H < dj.H {
		return true
	} else if di.H == dj.H {
		if di.L < dj.L {
			return true
		} else if di.L == dj.L {
			return a[i].Locator.Size < a[j].Locator.Size
		}
	}
	return false
}

// PullServers struct
// For a given under-replicated block, this structure represents which
// servers should pull the specified block and which servers they can
// pull it from.
type PullServers struct {
	To   []string // Servers that should pull the specified block
	From []string // Servers that already contain the specified block
}

// ComputePullServers creates a map from block locator to PullServers
// with one entry for each under-replicated block.
//
// This method ignores zero-replica blocks since there are no servers
// to pull them from, so callers should feel free to omit them, but
// this function will ignore them if they are provided.
func ComputePullServers(kc *keepclient.KeepClient,
	keepServerInfo *keep.ReadServers,
	blockToDesiredReplication map[blockdigest.DigestWithSize]int,
	underReplicated BlockSet) (m map[Locator]PullServers) {
	m = map[Locator]PullServers{}
	// We use CanonicalString to avoid filling memory with dupicate
	// copies of the same string.
	var cs CanonicalString

	// Servers that are writeable
	writableServers := map[string]struct{}{}
	for _, url := range kc.WritableLocalRoots() {
		writableServers[cs.Get(url)] = struct{}{}
	}

	for block := range underReplicated {
		serversStoringBlock := keepServerInfo.BlockToServers[block]
		numCopies := len(serversStoringBlock)
		numCopiesMissing := blockToDesiredReplication[block] - numCopies
		if numCopiesMissing > 0 {
			// We expect this to always be true, since the block was listed
			// in underReplicated.

			if numCopies > 0 {
				// Not much we can do with blocks with no copies.

				// A server's host-port string appears as a key in this map
				// iff it contains the block.
				serverHasBlock := map[string]struct{}{}
				for _, info := range serversStoringBlock {
					sa := keepServerInfo.KeepServerIndexToAddress[info.ServerIndex]
					serverHasBlock[cs.Get(sa.URL())] = struct{}{}
				}

				roots := keepclient.NewRootSorter(kc.LocalRoots(),
					block.String()).GetSortedRoots()

				l := Locator(block)
				m[l] = CreatePullServers(cs, serverHasBlock, writableServers,
					roots, numCopiesMissing)
			}
		}
	}
	return m
}

// CreatePullServers creates a pull list in which the To and From
// fields preserve the ordering of sorted servers and the contents
// are all canonical strings.
func CreatePullServers(cs CanonicalString,
	serverHasBlock map[string]struct{},
	writableServers map[string]struct{},
	sortedServers []string,
	maxToFields int) (ps PullServers) {

	ps = PullServers{
		To:   make([]string, 0, maxToFields),
		From: make([]string, 0, len(serverHasBlock)),
	}

	for _, host := range sortedServers {
		// Strip the protocol portion of the url.
		// Use the canonical copy of the string to avoid memory waste.
		server := cs.Get(host)
		_, hasBlock := serverHasBlock[server]
		if hasBlock {
			// The from field should include the protocol.
			ps.From = append(ps.From, cs.Get(host))
		} else if len(ps.To) < maxToFields {
			_, writable := writableServers[host]
			if writable {
				ps.To = append(ps.To, server)
			}
		}
	}

	return
}

// RemoveProtocolPrefix strips the protocol prefix from a url.
func RemoveProtocolPrefix(url string) string {
	return url[(strings.LastIndex(url, "/") + 1):]
}

// BuildPullLists produces a PullList for each keep server.
func BuildPullLists(lps map[Locator]PullServers) (spl map[string]PullList) {
	spl = map[string]PullList{}
	// We don't worry about canonicalizing our strings here, because we
	// assume lps was created by ComputePullServers() which already
	// canonicalized the strings for us.
	for locator, pullServers := range lps {
		for _, destination := range pullServers.To {
			pullList, pullListExists := spl[destination]
			if !pullListExists {
				pullList = PullList{}
			}
			spl[destination] = append(pullList,
				PullRequest{Locator: locator, Servers: pullServers.From})
		}
	}
	return
}

// WritePullLists writes each pull list to a file.
// The filename is based on the hostname.
//
// This is just a hack for prototyping, it is not expected to be used
// in production.
func WritePullLists(arvLogger *logger.Logger,
	pullLists map[string]PullList,
	dryRun bool) error {
	r := strings.NewReplacer(":", ".")

	for host, list := range pullLists {
		if arvLogger != nil {
			// We need a local variable because Update doesn't call our mutator func until later,
			// when our list variable might have been reused by the next loop iteration.
			host := host
			listLen := len(list)
			arvLogger.Update(func(p map[string]interface{}, e map[string]interface{}) {
				pullListInfo := logger.GetOrCreateMap(p, "pull_list_len")
				pullListInfo[host] = listLen
			})
		}

		if dryRun {
			log.Print("dry run, not sending pull list to service %s with %d blocks", host, len(list))
			continue
		}

		filename := fmt.Sprintf("pull_list.%s", r.Replace(RemoveProtocolPrefix(host)))
		pullListFile, err := os.Create(filename)
		if err != nil {
			return err
		}
		defer pullListFile.Close()

		enc := json.NewEncoder(pullListFile)
		err = enc.Encode(list)
		if err != nil {
			return err
		}
		log.Printf("Wrote pull list to %s.", filename)
	}

	return nil
}
