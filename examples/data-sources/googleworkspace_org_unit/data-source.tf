data "googleworkspace_org_unit" "board" {
  org_unit_path = "/SMI Board"
}

output "board_org_unit_id" {
  value = data.googleworkspace_org_unit.board.id
}
