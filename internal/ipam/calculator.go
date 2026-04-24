package ipam

import (
	"fmt"
	"math"
	"math/bits"
	"net/netip"
)

// RequiredPrefixLen returns the prefix length needed to hold at least ipCount IPs.
// e.g. 50 → /26 (64 IPs), 100 → /25 (128 IPs)
func RequiredPrefixLen(ipCount int) (int, int) {
	if ipCount <= 0 {
		return 32, 1
	}
	power := int(math.Ceil(math.Log2(float64(ipCount))))
	if power < 1 {
		power = 1
	}
	actual := 1 << power
	prefixLen := 32 - power
	if prefixLen < 0 {
		prefixLen = 0
	}
	return prefixLen, actual
}

// ParseCIDR parses a CIDR string to netip.Prefix (masked/canonical).
func ParseCIDR(cidr string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
	}
	return p.Masked(), nil
}

// ParseIP parses a single IPv4 address string.
func ParseIP(s string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, fmt.Errorf("invalid IP %q: %w", s, err)
	}
	if !addr.Is4() {
		return netip.Addr{}, fmt.Errorf("only IPv4 is supported, got: %s", s)
	}
	return addr, nil
}

// PrefixIPCount returns the total IP count of a prefix.
func PrefixIPCount(p netip.Prefix) uint64 {
	return 1 << (32 - p.Bits())
}

// RangeIPCount returns the IP count of a [startIP, endIP] inclusive range.
func RangeIPCount(startIP, endIP string) (uint64, error) {
	s, err := ParseIP(startIP)
	if err != nil {
		return 0, err
	}
	e, err := ParseIP(endIP)
	if err != nil {
		return 0, err
	}
	start := AddrToUint32(s)
	end := AddrToUint32(e)
	if end < start {
		return 0, fmt.Errorf("end IP %s is before start IP %s", endIP, startIP)
	}
	return uint64(end-start) + 1, nil
}

// PrefixContains checks if outer fully contains inner.
func PrefixContains(outer, inner netip.Prefix) bool {
	return outer.Contains(inner.Addr()) && outer.Bits() <= inner.Bits()
}

// PrefixOverlaps checks if two prefixes overlap.
func PrefixOverlaps(a, b netip.Prefix) bool {
	return a.Overlaps(b)
}

// AddrToUint32 converts a netip.Addr (IPv4) to uint32. Exported for handler use.
func AddrToUint32(a netip.Addr) uint32 {
	b := a.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

// Uint32ToAddr converts a uint32 to netip.Addr (IPv4). Exported for handler use.
func Uint32ToAddr(v uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v),
	})
}

// prefixRange returns the start and end (exclusive) IP of a prefix as uint32.
func prefixRange(p netip.Prefix) (uint32, uint32) {
	start := AddrToUint32(p.Masked().Addr())
	size := uint32(1) << (32 - p.Bits())
	return start, start + size
}

// PoolRange 描述一个网段池的 IP 范围 [Start, End)，End 为开区间。
// 不论池是用 CIDR 还是 IP 范围定义的，统一转换为此结构参与计算。
type PoolRange struct {
	Start uint32 // 起始 IP（含）
	End   uint32 // 结束 IP（不含，即实际最后一个 IP + 1）
}

// PoolRangeFromCIDR 从 CIDR 构造 PoolRange
func PoolRangeFromCIDR(cidr string) (PoolRange, error) {
	p, err := ParseCIDR(cidr)
	if err != nil {
		return PoolRange{}, err
	}
	s, e := prefixRange(p)
	return PoolRange{Start: s, End: e}, nil
}

// PoolRangeFromIPs 从起止 IP 字符串构造 PoolRange（EndIP 为闭区间，内部转为开区间）
func PoolRangeFromIPs(startIP, endIP string) (PoolRange, error) {
	s, err := ParseIP(startIP)
	if err != nil {
		return PoolRange{}, err
	}
	e, err := ParseIP(endIP)
	if err != nil {
		return PoolRange{}, err
	}
	start := AddrToUint32(s)
	end := AddrToUint32(e)
	if end < start {
		return PoolRange{}, fmt.Errorf("end IP %s is before start IP %s", endIP, startIP)
	}
	return PoolRange{Start: start, End: end + 1}, nil // +1 转为开区间
}

// IPCount 返回池的 IP 总数
func (pr PoolRange) IPCount() uint64 {
	return uint64(pr.End - pr.Start)
}

// FreeBlock represents a contiguous free IP range.
type FreeBlock struct {
	CIDR    string `json:"cidr"`
	IPCount uint64 `json:"ip_count"`
	StartIP string `json:"start_ip"`
	EndIP   string `json:"end_ip"`
}

// FindFreeBlocks calculates free (unallocated) blocks within a pool range.
// allocated 为已分配的 CIDR 前缀列表。
// Returns a list of free blocks expressed as the largest possible aligned CIDRs.
func FindFreeBlocks(pool PoolRange, allocated []netip.Prefix) []FreeBlock {
	type ipRange struct{ start, end uint32 }

	// Collect allocated ranges within this pool
	var ranges []ipRange
	for _, a := range allocated {
		s, e := prefixRange(a)
		if s < pool.End && e > pool.Start {
			if s < pool.Start {
				s = pool.Start
			}
			if e > pool.End {
				e = pool.End
			}
			ranges = append(ranges, ipRange{s, e})
		}
	}

	// Sort by start
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[j].start < ranges[i].start {
				ranges[i], ranges[j] = ranges[j], ranges[i]
			}
		}
	}

	// Merge overlapping
	var merged []ipRange
	for _, r := range ranges {
		if len(merged) > 0 && r.start <= merged[len(merged)-1].end {
			if r.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = r.end
			}
		} else {
			merged = append(merged, r)
		}
	}

	// Find free gaps
	var freeRanges []ipRange
	cursor := pool.Start
	for _, r := range merged {
		if r.start > cursor {
			freeRanges = append(freeRanges, ipRange{cursor, r.start})
		}
		if r.end > cursor {
			cursor = r.end
		}
	}
	if cursor < pool.End {
		freeRanges = append(freeRanges, ipRange{cursor, pool.End})
	}

	// Convert free ranges to aligned CIDR blocks
	var result []FreeBlock
	for _, fr := range freeRanges {
		cidrs := rangeToCIDRs(fr.start, fr.end)
		for _, c := range cidrs {
			count := PrefixIPCount(c)
			s, _ := prefixRange(c)
			result = append(result, FreeBlock{
				CIDR:    c.String(),
				IPCount: count,
				StartIP: Uint32ToAddr(s).String(),
				EndIP:   Uint32ToAddr(s + uint32(count) - 1).String(),
			})
		}
	}
	return result
}

// rangeToCIDRs converts a raw IP range [start, end) into a minimal set of aligned CIDR blocks.
func rangeToCIDRs(start, end uint32) []netip.Prefix {
	var result []netip.Prefix
	for start < end {
		maxBits := 32
		if start != 0 {
			trailingZeros := bits.TrailingZeros32(start)
			maxBits = 32 - trailingZeros
		} else {
			maxBits = 0
		}
		size := end - start
		sizeBits := 32 - bits.Len32(size-1)
		if sizeBits < 0 {
			sizeBits = 0
		}
		if sizeBits <= maxBits {
			maxBits = sizeBits
		}
		blockSize := uint32(1) << (32 - maxBits)
		for start+blockSize > end {
			maxBits++
			blockSize = uint32(1) << (32 - maxBits)
		}
		addr := Uint32ToAddr(start)
		prefix := netip.PrefixFrom(addr, maxBits)
		result = append(result, prefix)
		start += blockSize
	}
	return result
}

// AllocateSubnet finds the best-fit free block in a pool range for the requested IP count.
// Returns the allocated CIDR prefix and actual IP count.
func AllocateSubnet(pool PoolRange, allocated []netip.Prefix, requestedIPs int) (netip.Prefix, int, error) {
	prefixLen, actualIPs := RequiredPrefixLen(requestedIPs)
	blockSize := uint32(actualIPs)

	type ipRange struct{ start, end uint32 }

	// Collect and sort allocated ranges within pool
	var ranges []ipRange
	for _, a := range allocated {
		s, e := prefixRange(a)
		if s < pool.End && e > pool.Start {
			if s < pool.Start {
				s = pool.Start
			}
			if e > pool.End {
				e = pool.End
			}
			ranges = append(ranges, ipRange{s, e})
		}
	}
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[j].start < ranges[i].start {
				ranges[i], ranges[j] = ranges[j], ranges[i]
			}
		}
	}
	var merged []ipRange
	for _, r := range ranges {
		if len(merged) > 0 && r.start <= merged[len(merged)-1].end {
			if r.end > merged[len(merged)-1].end {
				merged[len(merged)-1].end = r.end
			}
		} else {
			merged = append(merged, r)
		}
	}

	// Find free gaps
	var freeGaps []ipRange
	cursor := pool.Start
	for _, r := range merged {
		if r.start > cursor {
			freeGaps = append(freeGaps, ipRange{cursor, r.start})
		}
		if r.end > cursor {
			cursor = r.end
		}
	}
	if cursor < pool.End {
		freeGaps = append(freeGaps, ipRange{cursor, pool.End})
	}

	// Best-fit: find smallest gap that can hold the aligned block
	var bestStart uint32
	var bestGapSize uint64
	found := false

	for _, gap := range freeGaps {
		alignedStart := gap.start
		remainder := alignedStart % blockSize
		if remainder != 0 {
			alignedStart = alignedStart + blockSize - remainder
		}
		if alignedStart+blockSize > gap.end {
			continue
		}
		gapSize := uint64(gap.end - gap.start)
		if !found || gapSize < bestGapSize {
			bestStart = alignedStart
			bestGapSize = gapSize
			found = true
		}
	}

	if !found {
		displayRange := fmt.Sprintf("%s - %s", Uint32ToAddr(pool.Start).String(), Uint32ToAddr(pool.End-1).String())
		return netip.Prefix{}, 0, fmt.Errorf("no free space in pool [%s] for %d IPs (need /%d block)", displayRange, requestedIPs, prefixLen)
	}

	addr := Uint32ToAddr(bestStart)
	result := netip.PrefixFrom(addr, prefixLen)
	return result, actualIPs, nil
}
