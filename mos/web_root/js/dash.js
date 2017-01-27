(function($) {
  var ws, pageCache = {};
  PNotify.prototype.options.styling = 'fontawesome';
  PNotify.prototype.options.delay = 5000;
  var getCookie = function(name) {
    var m = (document.cookie || '').match(new RegExp(name + '=([^;"\\s]+)'));
    return m ? m[1] : '';
  };
  $.ajaxSetup({type: 'POST'});

  $(document.body)
      .on('click', '.dropdown-menu li', function(ev) {
        var text = $(this).find('a').text();
        $(this)
            .closest('.input-group')
            .find('input')
            .val(text)
            .trigger('change');
      });

  $(document).ajaxSend(function(event, xhr, settings) {
    $('#top_nav').addClass('spinner');
  }).ajaxStop(function() {
    $('#top_nav').removeClass('spinner');
  }).ajaxComplete(function(event, xhr) {
    // $('#top_nav').removeClass('spinner');
    if (xhr.status == 200) return;
    new PNotify({
      title: 'Server Error',
      text: 'Reason: ' + (xhr.responseText || 'connection error'),
      type: 'error'
    });
  });

  $(document)
      .on('click', '#reboot-button', function() {
        $.ajax({url: '/call', data: {method: 'Sys.Reboot'}}).done(function() {
          new PNotify({title: 'Device rebooted', type: 'success'});
        });
      });

  $(document).on('click', '#clear-logs-button', function() {
    $('#device-logs').empty();
    new PNotify({title: 'Console cleared', type: 'success'});
  });

  var loadPage = function(page) {
    var doit = function(html) {
      $('#app_view').html(html);
      $('#breadcrumb').html($('[data-title]').attr('data-title'));
    };
    if (pageCache[page]) {
      doit(pageCache[page]);
    } else {
      $.get('page_' + page + '.html').done(function(html) {
        pageCache[page] = html;
        doit(html);
      });
    }
  };

  $(document).on('click', 'a[tab]', function() {
    var page = $(this).attr('tab');
    loadPage(page);
  });

  $(document).ready(function() {
    $('a[tab]').first().click();
  });

  // Let tool know the port we want to use
  $.ajax({url: '/connect', data: {port: getCookie('port')}});

  $('#app_view').resizable({
    handleSelector: ".splitter-horizontal",
    resizeWidth: false
  });

  $('#d1').height($(document.body).height() - 60);
  $('#app_view').height($(d1).height() * 0.75);
  $('#device-logs-panel').height($(d1).height() * 0.25);



  // Instance the tour
  var tour = new Tour({
    steps: [
    {
      element: '#device-logs',
      title: 'See device logs',
      placement: 'top',
      content: 'This panel shows device logs produced by the ' +
        'JavaScript code in <code>init.js</code>.'
    },
    {
      element: '.splitter-horizontal',
      placement: 'top',
      title: 'Resize panels',
      reflex: true,
      content: 'You can resize panels by dragging this resize handle.'
    },
    {
      element: '#file-list',
      title: 'Edit init.js',
      reflex: true,
      content: 'Click on <code>init.js</code> to edit it.'
    },
    {
      element: '#file-textarea',
      title: 'Modify code',
      placement: 'left',
      content: 'Change \'Tock\' to \'Boom\''
    },
    {
      element: '#file-save-button',
      title: 'Save File',
      placement: 'bottom',
      reflex: true,
      content: 'Click "Save selected file" button to save the modified file back to the device.'
    },
    {
      element: '#reboot-button',
      title: 'Reboot device',
      placement: 'top',
      reflex: true,
      content: 'Click on "Reboot device" button to re-evaluate <code>init.js</code>.'
    },
    {
      element: '#device-logs',
      title: 'See modified message',
      placement: 'top',
      reflex: true,
      content: 'Notice that printed message has changed.'
    },
    {
      element: '[tab=examples]',
      title: 'See JavaScript examples apps',
      placement: 'right',
      reflex: true,
      content: 'Click on examples tab to see a list of examples we have put ' +
      'together to demonstrate the power and simplicity of Mongoose OS.'
    },
    {
      element: '#example-list',
      title: 'Click on button_mqtt.js',
      reflex: true,
      content: 'Click on <code>button_mqtt.js</code>.'
    },
    {
      element: '#example-textarea',
      title: 'Copy the code',
      reflex: true,
      content: 'Select the example code with your mouse and type Ctrl-C to copy it into clipboard.'
    },
    {
      element: '[tab=files]',
      title: 'Switch to device files',
      reflex: true,
      content: 'Click on file manager tab to see device files again.'
    },
    {
      element: '#file-list',
      title: 'Edit init.js',
      content: 'Click on <code>init.js</code>, paste example code.'
    },
    {
      element: '#file-save-button',
      title: 'Save File',
      placement: 'bottom',
      reflex: true,
      content: 'Click "Save selected file" to save the file.'
    },
    {
      element: '#reboot-button',
      title: 'Reboot device',
      placement: 'top',
      reflex: true,
      content: 'Click on "Reboot device" button to re-evaluate <code>init.js</code>.'
    },
    {
      element: '#file-textarea',
      title: 'Login to MQTT client',
      placement: 'left',
      content: 'Follow the instructions written in code comments to log in ' +
      'to the MQTT client and send commands to your device over the network.'
    },

  ]});
  tour.init();
  tour.start();

  $(document).on('click', '#link-tour', function() {
    tour.restart();
  });

})(jQuery);
