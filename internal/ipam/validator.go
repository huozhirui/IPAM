package ipam

import (
	"fmt"
	"net/netip"
)

// ValidateCIDR checks if a CIDR string is valid IPv4.
func ValidateCIDR(cidr string) error {
	p, err := netip.ParsePrefix(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %w", err)
	}
	if !p.Addr().Is4() {
		return fmt.Errorf("only IPv4 is supported, got: %s", cidr)
	}
	if p.Masked() != p {
		return fmt.Errorf("CIDR %s is not in canonical form, did you mean %s?", cidr, p.Masked().String())
	}
	return nil
}

// ValidateSubnetInPool checks if a subnet CIDR falls entirely within the pool.
func ValidateSubnetInPool(poolCIDR, subnetCIDR string) error {
	pool, err := ParseCIDR(poolCIDR)
	if err != nil {
		return fmt.Errorf("invalid pool CIDR: %w", err)
	}
	subnet, err := ParseCIDR(subnetCIDR)
	if err != nil {
		return fmt.Errorf("invalid subnet CIDR: %w", err)
	}
	if !PrefixContains(pool, subnet) {
		return fmt.Errorf("subnet %s is not within pool %s", subnetCIDR, poolCIDR)
	}
	return nil
}

// CheckConflict checks if a new CIDR overlaps with any existing allocation.
func CheckConflict(newCIDR string, existingCIDRs []string) error {
	newPrefix, err := ParseCIDR(newCIDR)
	if err != nil {
		return err
	}
	for _, existing := range existingCIDRs {
		ep, err := ParseCIDR(existing)
		if err != nil {
			continue
		}
		if PrefixOverlaps(newPrefix, ep) {
			return fmt.Errorf("CIDR %s overlaps with existing allocation %s", newCIDR, existing)
		}
	}
	return nil
}
