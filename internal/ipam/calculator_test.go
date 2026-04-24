package ipam

import (
	"net/netip"
	"testing"
)

func TestRequiredPrefixLen(t *testing.T) {
	tests := []struct {
		ipCount    int
		wantPrefix int
		wantActual int
	}{
		{1, 31, 2},
		{2, 31, 2},
		{3, 30, 4},
		{50, 26, 64},
		{64, 26, 64},
		{100, 25, 128},
		{128, 25, 128},
		{200, 24, 256},
		{256, 24, 256},
	}
	for _, tt := range tests {
		prefix, actual := RequiredPrefixLen(tt.ipCount)
		if prefix != tt.wantPrefix || actual != tt.wantActual {
			t.Errorf("RequiredPrefixLen(%d) = (%d, %d), want (%d, %d)",
				tt.ipCount, prefix, actual, tt.wantPrefix, tt.wantActual)
		}
	}
}

// 辅助函数：从 CIDR 构造 PoolRange
func mustPoolRange(cidr string) PoolRange {
	pr, err := PoolRangeFromCIDR(cidr)
	if err != nil {
		panic(err)
	}
	return pr
}

func TestFindFreeBlocks(t *testing.T) {
	pool := mustPoolRange("10.0.0.0/24")
	alloc1, _ := ParseCIDR("10.0.0.0/26")  // 10.0.0.0 - 10.0.0.63
	alloc2, _ := ParseCIDR("10.0.0.128/25") // 10.0.0.128 - 10.0.0.255

	blocks := FindFreeBlocks(pool, []netip.Prefix{alloc1, alloc2})
	if len(blocks) != 1 {
		t.Fatalf("expected 1 free block, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].CIDR != "10.0.0.64/26" {
		t.Errorf("expected free block 10.0.0.64/26, got %s", blocks[0].CIDR)
	}
}

func TestFindFreeBlocksEmpty(t *testing.T) {
	pool := mustPoolRange("10.0.0.0/24")
	blocks := FindFreeBlocks(pool, nil)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 free block for empty pool, got %d", len(blocks))
	}
	if blocks[0].CIDR != "10.0.0.0/24" {
		t.Errorf("expected 10.0.0.0/24, got %s", blocks[0].CIDR)
	}
	if blocks[0].IPCount != 256 {
		t.Errorf("expected 256 IPs, got %d", blocks[0].IPCount)
	}
}

// 测试用 IP 范围定义的池（非 CIDR 对齐）
func TestFindFreeBlocksWithRange(t *testing.T) {
	// 范围池: 10.0.0.10 - 10.0.0.99 (90 个 IP)
	pool, _ := PoolRangeFromIPs("10.0.0.10", "10.0.0.99")
	blocks := FindFreeBlocks(pool, nil)

	totalFree := uint64(0)
	for _, b := range blocks {
		totalFree += b.IPCount
	}
	if totalFree != 90 {
		t.Errorf("expected 90 free IPs, got %d", totalFree)
	}
}

func TestAllocateSubnet(t *testing.T) {
	pool := mustPoolRange("10.0.0.0/24")
	alloc1, _ := ParseCIDR("10.0.0.0/26")

	result, actual, err := AllocateSubnet(pool, []netip.Prefix{alloc1}, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.String() != "10.0.0.64/26" {
		t.Errorf("expected 10.0.0.64/26, got %s", result.String())
	}
	if actual != 64 {
		t.Errorf("expected 64 actual IPs, got %d", actual)
	}
}

func TestAllocateSubnetNoSpace(t *testing.T) {
	pool := mustPoolRange("10.0.0.0/30")
	alloc1, _ := ParseCIDR("10.0.0.0/30")

	_, _, err := AllocateSubnet(pool, []netip.Prefix{alloc1}, 2)
	if err == nil {
		t.Fatal("expected error for full pool, got nil")
	}
}

// 测试范围池的分配
func TestAllocateSubnetWithRange(t *testing.T) {
	// 范围池: 10.0.0.0 - 10.0.0.255 (等价于 /24)
	pool, _ := PoolRangeFromIPs("10.0.0.0", "10.0.0.255")

	result, actual, err := AllocateSubnet(pool, nil, 50)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Bits() != 26 {
		t.Errorf("expected /26, got /%d", result.Bits())
	}
	if actual != 64 {
		t.Errorf("expected 64 actual IPs, got %d", actual)
	}
}

func TestRangeIPCount(t *testing.T) {
	count, err := RangeIPCount("10.0.0.0", "10.0.0.255")
	if err != nil {
		t.Fatal(err)
	}
	if count != 256 {
		t.Errorf("expected 256, got %d", count)
	}

	count, err = RangeIPCount("10.0.0.10", "10.0.0.99")
	if err != nil {
		t.Fatal(err)
	}
	if count != 90 {
		t.Errorf("expected 90, got %d", count)
	}
}

func TestCheckConflict(t *testing.T) {
	existing := []string{"10.0.0.0/26", "10.0.0.128/25"}

	err := CheckConflict("10.0.0.0/24", existing)
	if err == nil {
		t.Error("expected conflict error")
	}

	err = CheckConflict("10.0.0.64/26", existing)
	if err != nil {
		t.Errorf("unexpected conflict: %v", err)
	}
}
