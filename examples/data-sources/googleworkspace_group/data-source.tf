data "googleworkspace_group" "board" {
  email = "board@spokanemountaineers.org"
}

output "board_group_id" {
  value = data.googleworkspace_group.board.id
}
