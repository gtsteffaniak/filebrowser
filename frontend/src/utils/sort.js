import { state } from "@/store";

export function sortedItems(items = [], sortby="name") {
    return items.sort((a, b) => {
        let valueA = a[sortby];
        let valueB = b[sortby];

        if (sortby === "name") {
            valueA = valueA.split(".")[0]
            valueB = valueB.split(".")[0]
            // Handle sorting for "name" field
            const isNumericA = !isNaN(valueA);
            const isNumericB = !isNaN(valueB);

            if (isNumericA && isNumericB) {
                // Compare numeric strings as numbers
                return state.user.sorting.asc
                    ? parseFloat(valueA) - parseFloat(valueB)
                    : parseFloat(valueB) - parseFloat(valueA);
            }
            // Compare non-numeric values as strings
            return state.user.sorting.asc
                ? valueA.localeCompare(valueB)
                : valueB.localeCompare(valueA);
        }

        // Default sorting for other fields
        if (state.user.sorting.asc) {
            return valueA > valueB ? 1 : -1;
        } else {
            return valueA < valueB ? 1 : -1;
        }
    });
}
