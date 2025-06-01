on the CLI we want progress updates while working on particularly large BOMs

validate that PNs are unique

validate that the BOM doesn't contain cycles

instead of having "AlternateSelector" just pass the inventoryRepo and itemRepo throughout. Don't need those alternate_selector functions to be hanging off a struct for no reason.

why does bomrepository implement both itemrepository and bomrepository interfaces?