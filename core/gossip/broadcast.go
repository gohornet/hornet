package gossip

import (
	"github.com/gohornet/hornet/core/database"
	p2pcore "github.com/gohornet/hornet/core/p2p"
	"github.com/gohornet/hornet/pkg/model/milestone"
	"github.com/gohornet/hornet/pkg/p2p"
	"github.com/gohornet/hornet/pkg/protocol/gossip"
)

// BroadcastHeartbeat broadcasts a heartbeat message to every connected peer who supports STING.
func BroadcastHeartbeat(filter func(proto *gossip.Protocol) bool) {
	snapshotInfo := database.Tangle().GetSnapshotInfo()
	if snapshotInfo == nil {
		return
	}

	latestMilestoneIndex := database.Tangle().GetSolidMilestoneIndex()
	connectedCount := p2pcore.Manager().ConnectedCount(p2p.PeerRelationKnown)
	syncedCount := Service().SynchronizedCount(latestMilestoneIndex)
	// TODO: overflow not handled for synced/connected
	heartbeatMsg, _ := gossip.NewHeartbeatMsg(latestMilestoneIndex, snapshotInfo.PruningIndex, database.Tangle().GetLatestMilestoneIndex(), byte(connectedCount), byte(syncedCount))

	Service().ForEach(func(proto *gossip.Protocol) bool {
		if filter != nil && !filter(proto) {
			return true
		}
		proto.Enqueue(heartbeatMsg)
		return true
	})
}

// BroadcastMilestoneRequests broadcasts up to N requests for milestones nearest to the current solid milestone index
// to every connected peer who supports STING. Returns the number of milestones requested.
func BroadcastMilestoneRequests(rangeToRequest int, onExistingMilestoneInRange func(index milestone.Index), from ...milestone.Index) int {
	var requested int

	// make sure we only request what we don't have
	startingPoint := database.Tangle().GetSolidMilestoneIndex()
	if len(from) > 0 {
		startingPoint = from[0]
	}
	var msIndexes []milestone.Index
	for i := 1; i <= rangeToRequest; i++ {
		toReq := startingPoint + milestone.Index(i)
		// only request if we do not have the milestone
		if !database.Tangle().ContainsMilestone(toReq) {
			requested++
			msIndexes = append(msIndexes, toReq)
			continue
		}
		if onExistingMilestoneInRange != nil {
			onExistingMilestoneInRange(toReq)
		}
	}

	if len(msIndexes) == 0 {
		return requested
	}

	// send each ms request to a random peer who supports the message
	for _, msIndex := range msIndexes {
		Service().ForEach(func(proto *gossip.Protocol) bool {
			if !proto.HasDataForMilestone(msIndex) {
				return true
			}
			proto.SendMilestoneRequest(msIndex)
			return false
		})
	}
	return requested
}