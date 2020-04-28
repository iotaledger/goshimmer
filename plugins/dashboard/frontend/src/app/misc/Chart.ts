
// includes chart options to increase performance, by disabling chart animations
export var defaultChartOptions = {
    elements: {
        line: {
            tension: 0
        }
    },
    animation: {
        duration: 0
    },
    hover: {
        animationDuration: 0
    },
    responsiveAnimationDuration: 0
};