(function($) {

  var reconnect = function() {
    var url = 'ws://' + location.host + '/ws';
    ws = new WebSocket(url);
    ws.onopen = function(ev) {
      // console.log(ev);
      $('#lost-connection').fadeOut(300);
    };
    ws.onclose = function(ev) {
      console.log(ev);
      $('#lost-connection').fadeIn(300);
      setTimeout(reconnect, 1000);
    };
    ws.onmessage = function(ev) {
      var m = JSON.parse(ev.data || '');
      switch (m.cmd) {
        case 'console':
          $('#device-logs').each(function(i, el) {
            // console.log(el, m);
            el.innerHTML += m.data;
            el.scrollTop = el.scrollHeight;
          });
          break;
        case 'ports':
          var ports = (m.data || '').split(',');
          $('#dropdown-ports').empty();
          $.each(ports, function(i, v) {
            $('<li><a href="#">' + v + '</a></li>').appendTo('#dropdown-ports');
          });
          if (!$('#input-serial').val() && ports.length > 0) {
            $('#input-serial').val(ports[0]);
          }
          break;
        default:
          break;
      }
    };
    ws.onerror = function(ev) {
      console.log('error', ev);
      ws.close();
    };
  };
  reconnect();

})(jQuery);
