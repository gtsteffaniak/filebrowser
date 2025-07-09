export function sortedItems(items = [], sortby="name", asc=true) {
    return items.sort((a, b) => {
        let valueA = a[sortby];
        let valueB = b[sortby];

        if (sortby === "name") {
            // Use localeCompare with numeric option for natural sorting
            const comparison = valueA.localeCompare(valueB, undefined, { numeric: true, sensitivity: 'base' });
            return asc ? comparison : -comparison;
        }

        // Default sorting for other fields
        if (asc) {
            return valueA > valueB ? 1 : -1;
        } else {
            return valueA < valueB ? 1 : -1;
        }
    });
}
