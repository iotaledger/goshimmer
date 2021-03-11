export function displayManaUnit(mana: number): string {
    let result = ""
    // round to nearest integer
    let roundedMana = Math.round(mana);
    if (roundedMana < 1000) {
        result = roundedMana.toString(10) + " m"; // mana
    } else if (roundedMana < 1000000) {
        result = (roundedMana / 1000).toFixed(3) + " Km"; // kilomana
    }
    else if (roundedMana < 1000000000) {
        result = (roundedMana / 1000000).toFixed(3) + " Mm"; // megamana
    }
    else if (roundedMana < 1000000000000) {
        result = (roundedMana / 1000000000).toFixed(3) + " Gm"; // gigamana
    }
    else if (roundedMana < 1000000000000000) {
        result = (roundedMana / 1000000000000).toFixed(3) + " Tm"; // terramana
    } else {
        result = (roundedMana / 1000000000000000).toFixed(3) + " Pm"; // petamana
    }
    return result
}