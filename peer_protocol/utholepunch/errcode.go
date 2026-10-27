package utholepunch

type ErrCode uint32

const (
	NoSuchPeer ErrCode = iota + 1
	NotConnected
	NoSupport
	NoSelf
)

func (ec ErrCode) Error() string {
	switch ec {
	case NoSuchPeer:
		return "target endpoint is invalid"
	case NotConnected:
		return "not connected to the target peer"
	case NoSupport:
		return "target peer does not support ut_holepunch"
	case NoSelf:
		return "target endpoint belongs to the relaying peer"
	default:
		return "unknown ut_holepunch error"
	}
}
