var util = {
  generateUniqueID: function() {
    return Math.random().toString(36).substr(2, 9);
  },
  pubsub: function() {
    var events = {};
    return {
      subscribe: function(name, fn) {
        if (!events[name]) events[name] = [];
        events[name].push(fn);
      },
      unsubscribe: function(name, fn) {
        var index = (events[name] || []).indexOf(fn);
        if (index >= 0) events[name].splice(index, 1);
      },
      publish: function(name, data) {
        var arr = events[name];
        if (!arr) return;
        for (var i in arr.length) arr[i](data);
      },
    };
  },
  wsconnect: function(url, opts) {
    var wrapper = {
      callbacks: {},
      closed: false,
      close: function() {
        wrapper.closed = true;
        wrapper.ws.close();
      },
    };
    var reconnect = function() {
      var uriParams = '';
      if (opts && opts.auth) {
        var token = opts.auth();
        if (!token) {
          if (!wrapper.closed) wrapper.tid = setTimeout(reconnect, 1000);
        }
        uriParams = '?access_token=' + token;
      }

      var u = url;
      if (!u || !u.match(/^ws/)) {
        var proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
        u = proto + '//' + location.host + (u || '/ws') + uriParams;
      }
      // console.log('Reconnecting to', u);
      var ws = {};
      try {
        ws = new WebSocket(u);
      } catch (e) {
        console.log('Error creating websocket connection:', e);
      }
      ws.onmessage = function(ev) {
        var msg;
        try {
          msg = JSON.parse(ev.data);
        } catch (e) {
          console.log('Invalid ws frame:', ev.data);  // eslint-disable-line
        }
        if (msg) wrapper.onmessage(msg);  // Callback outside of try block
      };
      ws.onclose = function() {
        clearTimeout(wrapper.tid);
        if (!wrapper.closed) wrapper.tid = setTimeout(reconnect, 1000);
      };
      wrapper.ws = ws;
    };
    reconnect();
    return wrapper;
  },
};
