package ec2

import "testing"

// Security group rule literals shared across this file and sg_revoke_test.go.
const (
	formKeyIPProtocol1 = "IpPermissions.1.IpProtocol"
	formKeyCidrIP1     = "IpPermissions.1.IpRanges.1.CidrIp"
	cidrAllIPv4        = "0.0.0.0/0"
	cidrTenSlash8      = "10.0.0.0/8"
	protocolTCP        = "tcp"
)

// TestParseIPPermissionsFromForm covers the AWS Query wire shape for
// AuthorizeSecurityGroupIngress / AuthorizeSecurityGroupEgress. The
// terraform-aws provider sends these keys; without correct parsing the
// handler stored an empty IngressRules slice and `DescribeSecurityGroups`
// later surfaced `<ipPermissions></ipPermissions>` even though the
// caller said "open 22 to 0.0.0.0/0".
func TestParseIPPermissionsFromForm(t *testing.T) {
	form := map[string][]string{
		formKeyIPProtocol1:                  {protocolTCP},
		"IpPermissions.1.FromPort":          {"22"},
		"IpPermissions.1.ToPort":            {"22"},
		formKeyCidrIP1:                      {cidrAllIPv4},
		"IpPermissions.2.IpProtocol":        {"-1"},
		"IpPermissions.2.FromPort":          {"0"},
		"IpPermissions.2.ToPort":            {"0"},
		"IpPermissions.2.IpRanges.1.CidrIp": {cidrTenSlash8},
		// Unrelated keys don't pollute the parser.
		"GroupId": {"sg-test"},
	}

	perms := parseIPPermissionsFromForm(form)
	if got := len(perms); got != 2 {
		t.Fatalf("len(perms) = %d, want 2", got)
	}

	if got := perms[0]; got.IPProtocol != protocolTCP || got.FromPort != 22 || got.ToPort != 22 {
		t.Errorf("perms[0] = %+v, want tcp 22→22", got)
	}

	if len(perms[0].IPRanges) != 1 || perms[0].IPRanges[0].CidrIP != cidrAllIPv4 {
		t.Errorf("perms[0].IPRanges = %+v, want one 0.0.0.0/0", perms[0].IPRanges)
	}

	if got := perms[1]; got.IPProtocol != "-1" {
		t.Errorf("perms[1].IPProtocol = %q, want -1", got.IPProtocol)
	}
}

// TestParseIPPermissionsFromForm_OutOfOrder confirms we sort by N so
// member.2 supplied before member.1 still lands at the right slice
// position.
func TestParseIPPermissionsFromForm_OutOfOrder(t *testing.T) {
	form := map[string][]string{
		"IpPermissions.2.IpProtocol":        {"udp"},
		"IpPermissions.2.IpRanges.1.CidrIp": {cidrTenSlash8},
		formKeyIPProtocol1:                  {protocolTCP},
		formKeyCidrIP1:                      {cidrAllIPv4},
	}

	perms := parseIPPermissionsFromForm(form)
	if len(perms) != 2 {
		t.Fatalf("len(perms) = %d, want 2", len(perms))
	}

	if perms[0].IPProtocol != protocolTCP || perms[1].IPProtocol != "udp" {
		t.Errorf("expected tcp, udp; got %s, %s", perms[0].IPProtocol, perms[1].IPProtocol)
	}
}

// TestParseIPPermissionsFromForm_MultipleIpRanges checks the M-index
// IpRanges.M.CidrIp loop appends every range, not just the first.
func TestParseIPPermissionsFromForm_MultipleIpRanges(t *testing.T) {
	form := map[string][]string{
		formKeyIPProtocol1:                  {protocolTCP},
		"IpPermissions.1.FromPort":          {"443"},
		"IpPermissions.1.ToPort":            {"443"},
		formKeyCidrIP1:                      {cidrTenSlash8},
		"IpPermissions.1.IpRanges.2.CidrIp": {"172.16.0.0/12"},
		"IpPermissions.1.IpRanges.3.CidrIp": {"192.168.0.0/16"},
	}

	perms := parseIPPermissionsFromForm(form)
	if len(perms) != 1 {
		t.Fatalf("len(perms) = %d, want 1", len(perms))
	}

	if got := len(perms[0].IPRanges); got != 3 {
		t.Errorf("IPRanges count = %d, want 3", got)
	}
}

// TestParseIPPermissionsFromForm_EmptyForm returns nil safely.
func TestParseIPPermissionsFromForm_EmptyForm(t *testing.T) {
	perms := parseIPPermissionsFromForm(map[string][]string{})
	if len(perms) != 0 {
		t.Errorf("empty form should yield no permissions; got %d", len(perms))
	}
}
