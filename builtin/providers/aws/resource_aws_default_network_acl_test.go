package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/terraform"
)

func TestAccAWSDefaultNetworkAcl_basic(t *testing.T) {
	var networkAcl ec2.NetworkAcl

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSDefaultNetworkAclDestroy,
		Steps: []resource.TestStep{
			// Tests that a default_network_acl will show a non-empty plan if no rules are
			// given, indicating that it wants to destroy the default rules
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_basic,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
				),
				ExpectNonEmptyPlan: true,
			},
			// Add default ACL rules and veryify plan is empty
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_basicDefaultRules,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
				),
				ExpectNonEmptyPlan: false,
			},
		},
	})
}

func TestAccAWSDefaultNetworkAcl_deny(t *testing.T) {
	var networkAcl ec2.NetworkAcl

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSDefaultNetworkAclDestroy,
		Steps: []resource.TestStep{
			// Tests that a default_network_acl will show a non-empty plan if no rules are
			// given, indicating that it wants to destroy the default rules
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_deny,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
					testAccCheckAWSDefaultACLAttributes(&networkAcl, []*ec2.NetworkAclEntry{}, 0),
				),
			},
		},
	})
}

func TestAccAWSDefaultNetworkAcl_deny_ingress(t *testing.T) {
	// TestAccAWSDefaultNetworkAcl_deny_ingress will deny all Ingress rules, but
	// not Egress. We then expect there to be 3 rules, 2 AWS defaults and 1
	// additional Egress. Without specifying the Egress rule in the configuration,
	// we expect a follow up plan to prompt it's removal, thus we expect a non
	// emtpy plan
	var networkAcl ec2.NetworkAcl

	defaultEgressAcl := &ec2.NetworkAclEntry{
		CidrBlock:  aws.String("0.0.0.0/0"),
		Egress:     aws.Bool(true),
		Protocol:   aws.String("-1"),
		RuleAction: aws.String("allow"),
		RuleNumber: aws.Int64(100),
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSDefaultNetworkAclDestroy,
		Steps: []resource.TestStep{
			// Tests that a default_network_acl will show a non-empty plan if no rules are
			// given, indicating that it wants to destroy the default rules
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_deny_ingress,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
					testAccCheckAWSDefaultACLAttributes(&networkAcl, []*ec2.NetworkAclEntry{defaultEgressAcl}, 0),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSDefaultNetworkAcl_SubnetRemoval(t *testing.T) {
	var networkAcl ec2.NetworkAcl

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSDefaultNetworkAclDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_Subnets,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
					testAccCheckAWSDefaultACLAttributes(&networkAcl, []*ec2.NetworkAclEntry{}, 2),
				),
			},

			// Here the Subnets have been removed from the Default Network ACL Config,
			// but have not been reassigned. The result is that the Subnets are still
			// there, and we have a non-empty plan
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_Subnets_remove,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
					testAccCheckAWSDefaultACLAttributes(&networkAcl, []*ec2.NetworkAclEntry{}, 2),
				),
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAccAWSDefaultNetworkAcl_SubnetReassign(t *testing.T) {
	var networkAcl ec2.NetworkAcl

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAWSDefaultNetworkAclDestroy,
		Steps: []resource.TestStep{
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_Subnets,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
					testAccCheckAWSDefaultACLAttributes(&networkAcl, []*ec2.NetworkAclEntry{}, 2),
				),
			},

			// Here we've re-assinged the subnets to a different ACL, however, we
			// still arn't updating the Default resource, so we introduce a depends_on
			// reference, to ensure the right order
			resource.TestStep{
				Config: testAccAWSDefaultNetworkConfig_Subnets_move,
				Check: resource.ComposeTestCheckFunc(
					testAccGetWSDefaultNetworkAcl("aws_default_network_acl.default", &networkAcl),
					testAccCheckAWSDefaultACLAttributes(&networkAcl, []*ec2.NetworkAclEntry{}, 0),
				),
			},
		},
	})
}

func testAccCheckAWSDefaultNetworkAclDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_default_network_acl" {
			continue
		}
	}

	return nil
}

func testAccCheckAWSDefaultACLAttributes(acl *ec2.NetworkAcl, rules []*ec2.NetworkAclEntry, subnetCount int) resource.TestCheckFunc {
	return func(s *terraform.State) error {

		aclEntriesCount := len(acl.Entries)
		ruleCount := len(rules)

		// Default ACL has 2 hidden rules we can't do anything about
		ruleCount = ruleCount + 2

		if ruleCount != aclEntriesCount {
			return fmt.Errorf("Expected (%d) Rules, got (%d)", ruleCount, aclEntriesCount)
		}

		if len(acl.Associations) != subnetCount {
			return fmt.Errorf("Expected (%d) Subnets, got (%d)", subnetCount, len(acl.Associations))
		}

		return nil
	}
}

func testAccGetWSDefaultNetworkAcl(n string, networkAcl *ec2.NetworkAcl) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[n]
		if !ok {
			return fmt.Errorf("Not found: %s", n)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("No Network ACL is set")
		}
		conn := testAccProvider.Meta().(*AWSClient).ec2conn

		resp, err := conn.DescribeNetworkAcls(&ec2.DescribeNetworkAclsInput{
			NetworkAclIds: []*string{aws.String(rs.Primary.ID)},
		})
		if err != nil {
			return err
		}

		if len(resp.NetworkAcls) > 0 && *resp.NetworkAcls[0].NetworkAclId == rs.Primary.ID {
			*networkAcl = *resp.NetworkAcls[0]
			return nil
		}

		return fmt.Errorf("Network Acls not found")
	}
}

const testAccAWSDefaultNetworkConfig_basic = `
resource "aws_vpc" "tftestvpc" {
  cidr_block = "10.1.0.0/16"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}

resource "aws_default_network_acl" "default" {
  default_network_acl_id = "${aws_vpc.tftestvpc.default_network_acl_id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}
`

const testAccAWSDefaultNetworkConfig_basicDefaultRules = `
resource "aws_vpc" "tftestvpc" {
  cidr_block = "10.1.0.0/16"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}

resource "aws_default_network_acl" "default" {
  default_network_acl_id = "${aws_vpc.tftestvpc.default_network_acl_id}"

  ingress {
    protocol   = -1
    rule_no    = 100
    action     = "allow"
    cidr_block = "0.0.0.0/0"
    from_port  = 0
    to_port    = 0
  }

  egress {
    protocol   = -1
    rule_no    = 100
    action     = "allow"
    cidr_block = "0.0.0.0/0"
    from_port  = 0
    to_port    = 0
  }

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}
`

const testAccAWSDefaultNetworkConfig_deny = `
resource "aws_vpc" "tftestvpc" {
  cidr_block = "10.1.0.0/16"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}

resource "aws_default_network_acl" "default" {
  default_network_acl_id = "${aws_vpc.tftestvpc.default_network_acl_id}"

	ingress_deny = true
	egress_deny  = true

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}
`

const testAccAWSDefaultNetworkConfig_deny_ingress = `
resource "aws_vpc" "tftestvpc" {
  cidr_block = "10.1.0.0/16"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}

resource "aws_default_network_acl" "default" {
  default_network_acl_id = "${aws_vpc.tftestvpc.default_network_acl_id}"

	ingress_deny = true

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_basic"
  }
}
`

const testAccAWSDefaultNetworkConfig_Subnets = `
resource "aws_vpc" "foo" {
  cidr_block = "10.1.0.0/16"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_subnet" "one" {
  cidr_block = "10.1.111.0/24"
  vpc_id     = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_subnet" "two" {
  cidr_block = "10.1.1.0/24"
  vpc_id     = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_network_acl" "bar" {
  vpc_id = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_default_network_acl" "default" {
  default_network_acl_id = "${aws_vpc.foo.default_network_acl_id}"

  subnet_ids = ["${aws_subnet.one.id}", "${aws_subnet.two.id}"]

	ingress_deny = true
	egress_deny = true

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}
`

const testAccAWSDefaultNetworkConfig_Subnets_remove = `
resource "aws_vpc" "foo" {
  cidr_block = "10.1.0.0/16"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_subnet" "one" {
  cidr_block = "10.1.111.0/24"
  vpc_id     = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_subnet" "two" {
  cidr_block = "10.1.1.0/24"
  vpc_id     = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_network_acl" "bar" {
  vpc_id = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_default_network_acl" "default" {
  default_network_acl_id = "${aws_vpc.foo.default_network_acl_id}"

	ingress_deny = true
	egress_deny = true

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}
`

const testAccAWSDefaultNetworkConfig_Subnets_move = `
resource "aws_vpc" "foo" {
  cidr_block = "10.1.0.0/16"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_subnet" "one" {
  cidr_block = "10.1.111.0/24"
  vpc_id     = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_subnet" "two" {
  cidr_block = "10.1.1.0/24"
  vpc_id     = "${aws_vpc.foo.id}"

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_network_acl" "bar" {
  vpc_id = "${aws_vpc.foo.id}"

  subnet_ids = ["${aws_subnet.one.id}", "${aws_subnet.two.id}"]

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}

resource "aws_default_network_acl" "default" {
  default_network_acl_id = "${aws_vpc.foo.default_network_acl_id}"

  depends_on = ["aws_network_acl.bar"]

	ingress_deny = true
	egress_deny = true

  tags {
    Name = "TestAccAWSDefaultNetworkAcl_SubnetRemoval"
  }
}
`
