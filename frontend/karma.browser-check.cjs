module.exports = function(config) {
  config.set({
    frameworks: ['jasmine', '@angular-devkit/build-angular'],
    plugins: [require('karma-jasmine'), require('karma-chrome-launcher'), require('@angular-devkit/build-angular/plugins/karma')],
    customLaunchers: { BrowserCheck: { base: 'ChromeHeadless', flags: ['--disable-gpu', '--no-sandbox', '--disable-dev-shm-usage'] } },
    browsers: ['BrowserCheck'], singleRun: true, reporters: ['progress']
  });
};
