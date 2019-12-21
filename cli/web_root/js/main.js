'use strict';
var h = preact.h;

var app = null;  // App initialization will set this value
var maxLogMessages = 200000;
var maxHistory = 100;

var boards = {
  'STMicroelectronics': false,
  'STM32 B-L475E-IOT01A': '--platform stm32 --build-var BOARD=B-L475E-IOT01A',
  'STM32 DISCO-F746NG': '--platform stm32 --build-var BOARD=DISCO-F746NG',
  'STM32 NUCLEO-F746ZG': '--platform stm32 --build-var BOARD=NUCLEO-F746ZG',
  'Texas Instruments': false,
  'TI CC3220': '--platform cc3220',
  'TI CC3200': '--platform cc3200',
  'Espressif Systems': false,
  'ESP32': '--platform esp32',
  'ESP32 Olimex EVB': '--platform esp32 --build-var BOARD=ESP32-EVB',
  'ESP8266': '--platform esp8266',
  'ESP8266, flash 1M': '--platform esp8266 --build-var BOARD=esp8266-1M',
  'ESP8266, flash 2M': '--platform esp8266 --build-var BOARD=esp8266-2M',
};

var shortcuts = {
  n: {
    descr: 'Create new app',
    cmd: 'mos clone https://github.com/mongoose-os-apps/demo-js app1'
  },
  i: {descr: 'Show device info', cmd: 'mos call Sys.GetInfo', go: true},
  u: {descr: 'Reboot device', cmd: 'mos call Sys.Reboot', go: true},
  c: {
    descr: 'Call RPC service',
    cmd: 'mos call RPC.List \'{"param":"value"}\''
  },
  l: {descr: 'Reload window', cmd: 'reload', go: true},
};

var createClass = function(obj) {
  function F() {
    preact.Component.apply(this, arguments);
    if (obj.init) obj.init.call(this, this.props);
  }
  var p = F.prototype = Object.create(preact.Component.prototype);
  for (var i in obj) p[i] = obj[i];
  return p.constructor = F;
};

var map = function(arr, f) {
  var newarr = [];
  for (var i = 0; i < arr.length; i++) newarr.push(f(arr[i], i, arr));
  return newarr;
};

var mkref = function(key) {
  return function(el) {
    app[key] = el;
  };
};

var focusCommandInput = function() {
  setTimeout(function() {
    var el = (app || {}).commandInput;
    if (!el) return;
    el.focus();
    el.selectionStart = el.selectionEnd = el.value.length;
  }, 1);
};

var scrollToBottomIgnoreAutoscroll = function(el) {
  if (!el) return;
  setTimeout(function() {
    var scrollHeight = el.scrollHeight;
    var height = el.clientHeight;
    var maxScrollTop = scrollHeight - height;
    el.scrollTop = maxScrollTop > 0 ? maxScrollTop : 0;
  }, 1);
};

var scrollToBottom = function(el) {
  if (!app.state.autoscroll) return;
  scrollToBottomIgnoreAutoscroll(el);
};

var setBoard = function(value) {
  if (value.match(/^Clear/)) value = '';
  app.ls.mos_board = value;
  app.setState({board: value});
};

var tsURL = function(url) {
  return url + '?' + (+new Date());
};

var runCommand = function(cmd, nohistory) {
  app.setState({command: cmd, autoscroll: true, showHistory: false});
  if (!cmd || cmd.match(/^\s*$/)) return;
  if (cmd === 'reload') {
    location.reload();
    return;
  }

  // Save command to history
  if (!nohistory) {
    var hist = app.state.history;
    while (true) {
      var i = hist.indexOf(cmd);
      if (i < 0) break;
      hist.splice(i, 1);
    }
    hist.push(cmd);
    if (hist.length > maxHistory) hist.splice(0, hist.length - maxHistory);
    app.ls.mos_history = hist.join(',');
  }

  // Update `mos build` command, by adding arch-specific flags
  if (cmd.match(/^mos\s+build/i) && !cmd.match(/--platform/) &&
      app.state.board) {
    cmd += ' ' + boards[app.state.board];
  }

  // Display command in the messages window
  app.state.messages.push(
      h('hr', {class: 'my-2'}),
      h('span', {class: 'text-success'}, '$ ', cmd, '\n'));
  app.setState(app.state);  // to refresh the command log window
  scrollToBottom(app.logWindow);
  app.setState({command: '', busy: true, historyPos: 0});

  // Send command to mos binary
  var data = 'cmd=' + encodeURIComponent(cmd);
  return axios({
           url: tsURL('/terminal'),
           method: 'post',
           data: data,
           timeout: 3600000
         })
      .catch(function(err) {
        var obj = (err.response || {}).data || {};
        var message = obj.error || JSON.stringify(err);
        app.state.messages.push(
            h('span', {class: 'text-danger'}, message, '\n'));
        app.setState(app.state);
        scrollToBottom(app.logWindow);
      })
      .then(function(res) {
        app.state.messages.push(
            h('span', {class: 'text-success'}, 'Command completed.\n'));
        app.setState({busy: false});
        focusCommandInput();
        scrollToBottom(app.logWindow);
        if (cmd.match(/^cd\s/i) && res && res.data) {
          var cwd = res.data.result.replace(/\\/g, '/');
          app.setState({cwd: cwd});
          app.ls.mos_cwd = app.state.cwd;
        }
        var m = cmd.match(/^mos\s+clone\s+(\S+)\s+(.+)/i);
        if (m) runCommand('cd "' + m[2] + '"', true);
        return res;
      });
};

var History = function(props) {
  if (!app.state.showHistory) return '';
  return h(
      'div', {
        ref: mkref('historyWindow'),
        class: 'w-75 oa position-absolute',
        style: 'max-height: 15em; bottom: 100%;',
      },
      map(app.state.history, function(cmd, i) {
        var pos = app.state.history.length - app.state.historyPos;
        var opts = {
          class: 'border mw-100 oa m-0 p-0',
          onClick: function() {
            app.setState({showHistory: false, historyPos: 0, command: cmd});
            focusCommandInput();
          },
        };
        opts.class += i === pos ? ' alert-info' : ' alert-warning';
        return h('div', opts, cmd);
      }));
};

var Prompt = function(props) {
  var input = h('input', {
    class: 'form-control text-monospace',
    type: 'text',
    autofocus: true,
    placeholder: 'type command, hit enter',
    value: app.state.command,
    disabled: app.state.busy,
    onKeyDown: app.onKeyDown,
    autocomplete: 'off',    // safari
    autocorrect: 'off',     // needs
    autocapitalize: 'off',  // that to turn
    spellcheck: false,      // off autocorrection
    onInput: function(ev) {
      app.setState({command: ev.target.value});
    },
    ref: function(el) {
      app.commandInput = el;
    },
  });
  var progress =
      h('div', {class: 'progress form-control form-control-sm'}, h('div', {
          class: 'progress-bar progress-bar-striped progress-bar-animated ' +
              'w-100 bg-secondary',
          role: 'progressbar'
        }));
  var icon = h('img', {
    style: 'cursor: pointer;',
    width: 20,
    height: 20,
    src: 'data:image/svg+xml;base64,PD94bWwgdmVyc2lvbj0iMS4wIiA/PjxzdmcgZGF0Y' +
        'S1uYW1lPSJMYXllciAxIiBpZD0iTGF5ZXJfMSIgdmlld0JveD0iMCAwIDY0IDY0IiB4' +
        'bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHhtbG5zOnhsaW5rPSJodHR' +
        'wOi8vd3d3LnczLm9yZy8xOTk5L3hsaW5rIj48ZGVmcz48c3R5bGU+LmNscy0xe2ZpbG' +
        'w6dXJsKCNsaW5lYXItZ3JhZGllbnQpO30uY2xzLTJ7ZmlsbDojZmZmO308L3N0eWxlP' +
        'jxsaW5lYXJHcmFkaWVudCBncmFkaWVudFVuaXRzPSJ1c2VyU3BhY2VPblVzZSIgaWQ9' +
        'ImxpbmVhci1ncmFkaWVudCIgeDE9IjMiIHgyPSI2MSIgeTE9IjMyIiB5Mj0iMzIiPjx' +
        'zdG9wIG9mZnNldD0iMCIgc3RvcC1jb2xvcj0iI2ZiYjA0MCIvPjxzdG9wIG9mZnNldD' +
        '0iMSIgc3RvcC1jb2xvcj0iI2Y3OTQxZSIvPjwvbGluZWFyR3JhZGllbnQ+PC9kZWZzP' +
        'jx0aXRsZS8+PHJlY3QgY2xhc3M9ImNscy0xIiBoZWlnaHQ9IjU4IiByeD0iMTYuMDIi' +
        'IHJ5PSIxNi4wMiIgd2lkdGg9IjU4IiB4PSIzIiB5PSIzIi8+PHBhdGggY2xhc3M9ImN' +
        'scy0yIiBkPSJNNTEuNDgsMjAuMTNIMzAuMzJBNy41OSw3LjU5LDAsMCwxLDM3LjQ4LD' +
        'E1aDYuODNBNy42LDcuNiwwLDAsMSw1MS40OCwyMC4xM1oiLz48cGF0aCBjbGFzcz0iY' +
        '2xzLTIiIGQ9Ik01MS45LDQxLjQyQTcuNjEsNy42MSwwLDAsMSw0NC4zMSw0OUgxOS42' +
        'NWE3LjYxLDcuNjEsMCwwLDEtNy41OS03LjU4VjI5LjkyYTcuNjIsNy42MiwwLDAsMSw' +
        '3LjU5LTcuNTloNy44M3YwbDI0LjQ2LDBaIi8+PC9zdmc+',
  });
  var button =
      h('button', {
        onClick: function() {
          axios.post(
              tsURL('/open'), 'cmd=' + encodeURIComponent(app.state.cwd));
        },
        disabled: app.state.busy,
        class: 'btn btn-sm btn-outline-secondary',
      },
        icon);
  var label =
      h('div', {class: 'input-group-prepend'}, button,
        h('span', {class: 'input-group-text'}, app.state.cwd));
  return h(
      'div', {
        class: 'input-group input-group-sm position-relative',
        ref: mkref('elFooter')
      },
      h(History), label, app.state.busy ? progress : input);
};

var Dropdown = createClass({
  init: function(props) {
    this.state = {
      expanded: false,
    };
  },
  render: function(props, state) {
    var self = this;
    var cls = state.expanded ? ' show' : '';
    var items = map(props.children, function(value) {
      if (typeof (value) !== 'string') return value;
      return h(
          'a', {
            class: 'dropdown-item my-0 py-0 small',
            style: 'cursor:pointer;',
            href: '#',
            onClick: function() {
              self.setState({expanded: false});
              if (props.onSelect) props.onSelect(value);
              focusCommandInput();
            },
          },
          value);
    });
    return h(
        'div', {class: 'dropdown' + cls}, h('button', {
          disabled: props.disabled,
          class: 'btn btn-sm dropdown-toggle btn-outline-secondary',
          onClick: function() {
            self.setState({expanded: !state.expanded});
          },
        }),
        h('div', {class: 'dropdown-menu dropdown-menu-right' + cls}, items));
  },
});

var mklink = function(url, title) {
  return h(
      'a', {
        href: url,
        target: '_blank',
        class: 'text-nowrap',
        onClick: function(ev) {
          ev.preventDefault();
          axios.post(tsURL('/open'), 'cmd=' + encodeURIComponent(url));
        },
      },
      title);
};

var Header = function() {
  var sep = h('span', {class: 'mx-1'}, '|');
  var links =
      h('div', {class: 'float-right mt-1 text-muted'},
        h('span', {class: 'd-none d-lg-inline'},
          mklink(
              'https://mongoose-os.com/docs/mongoose-os/quickstart/setup.md',
              'docs'),
          sep,
          mklink(
              'https://www.youtube.com/channel/UCZ9lQ7b-4bDbLOLpKwjpSAw/videos',
              'youtube'),
          sep, mklink('https://community.mongoose-os.com', 'forum'), sep,
          mklink('http://dash.mongoose-os.com', 'mDash')));

  var portDropdown =
      h(Dropdown, {
        disabled: app.state.busy,
        onSelect: function(value) {
          if (value.match(/^Clear/)) value = '';
          app.setState({port: value});
        },
      },
        app.state.ports, h('h6', {class: 'dropdown-divider'}), 'Clear');

  var boardKeys = [];
  for (var k in boards) {
    if (boards[k]) boardKeys.push(k);
    if (!boards[k]) boardKeys.push(h('h6', {class: 'dropdown-header'}, k));
  }
  var boardsDropdown = h(
      Dropdown, {onSelect: setBoard, disabled: app.state.busy}, boardKeys,
      h('h6', {disabled: app.state.busy, class: 'dropdown-divider'}), 'Clear');

  var mkSelector = function(caption, dropdown, key) {
    return h(
        'span', {class: 'form-inline d-inline-block'},
        h('div', {class: 'input-group'}, h('input', {
            class: 'form-control form-control-sm small',
            type: 'text',
            disabled: app.state.busy,
            placeholder: caption,
            value: app.state[key],
            onInput: function(ev) {
              var update = {};
              update[key] = ev.target.value;
              app.setState(update);
            },
          }),
          h('div', {class: 'input-group-append'}, dropdown)));
  };

  var portSelector = mkSelector('Choose port', portDropdown, 'port');
  var boardsSelector = mkSelector('Choose board', boardsDropdown, 'board');
  var spacer = h('span', {class: 'mx-1'}, ' ');

  var refreshButton =
      h('div', {class: 'form-inline d-inline-block'},
        h('div', {class: 'form-group'},
          h('button', {
            class: 'btn btn-sm btn-outline-secondary',
            disabled: app.state.busy,
            onClick: function(ev) {
              location.reload();
            }
          },
            'reload window')));

  var wrapCheckbox =
      h('div', {class: 'form-inline d-inline-block'},
        h('div', {class: 'form-group'},
          h('div', {class: 'custom-control custom-checkbox'}, h('input', {
              type: 'checkbox',
              class: 'custom-control-input',
              id: 'cb_wrap',
              checked: !!app.state.wrap,
              onChange: function(ev) {
                app.setState({wrap: ev.target.checked});
                app.ls.mos_wrap = app.state.wrap ? 1 : '';
              },
            }),
            h('label', {'for': 'cb_wrap', class: 'custom-control-label'},
              'wrap lines'))));

  return h(
      'div', {ref: mkref('elHeader')}, portSelector, spacer, boardsSelector,
      spacer, refreshButton, spacer, wrapCheckbox, links);
};

var checkPorts = function() {
  axios.get(tsURL('/getports')).then(function(res) {
    var ports = res.data.result.Ports || [];
    app.setState({ports: ports});
    if (ports.length) {
      app.state.serial.push('Ports available:\n', ports.join('\n'), '\n\n');
    } else {
      app.state.serial.push(h(
          'span', {class: 'text-danger'},
          'No serial ports available. Possible reasons:\n',
          '  - A device is disconnected. Connect and press Ctrl-l.\n',
          '  - A USB-To-Serial driver is not installed. ', 'To install, see ',
          mklink(
              'https://mongoose-os.com/docs/mongoose-os/quickstart/setup.md#3-usb-to-serial-drivers',
              'instructions'),
          '.\n', '    When done, restart this tool.\n\n'));
    }
    app.setState(app.state);
    scrollToBottom(app.serialWindow);
  });
};

var HelpMessage = function() {
  var img = {src: 'images/logo.png', height: 32, width: 32, class: 'mb-2 mr-2'};
  var list = [];
  for (var key in shortcuts) {
    var s = shortcuts[key];
    list.push(
        h('b', {class: 'text-monospace text-info'}, '  Ctrl-' + key), '  ',
        s.descr, '\n');
  }
  return h(
      'div', {class: 'text-muted mx-0 my-1 p-0'}, h('img', img),
      'Mongoose OS\n', 'Welcome to the mos tool!\n', 'New user? Follow the ',
      mklink(
          'https://mongoose-os.com/docs/mongoose-os/quickstart/setup.md',
          'quickstart guide'),
      '\n', 'Experienced? Follow the ',
      mklink(
          'https://mongoose-os.com/docs/mongoose-os/quickstart/develop-in-c.md',
          'advanced usage guide'),
      '\n\n', 'Enter any mos command, e.g.: "mos help"\n',
      'or any system command, e.g.: "cd c:/mos" or "ls -l"\n',
      'Some commands have keyboard shortcuts:\n', list);
};

var LogWindow = function() {
  return createClass({
    foo: function() {
      console.log('foo called');
    }
  });
};

var App = createClass({
  init: function() {
    this.ls = window.localStorage || {};
    this.state = {
      history: (this.ls.mos_history || '').split(','),
      historyPos: 0,
      command: '',
      busy: false,
      ports: [],
      port: this.ls.mos_port || '',
      board: this.ls.mos_board || '',
      version: '',
      home: '',
      wrap: this.ls.mos_wrap,
      cwd: this.ls.mos_cwd || '',
      mwh: 200,
      serial: ['Serial console\n'],
      messages: [h(HelpMessage)],
      autoscroll: true,
      showHistory: false,
    };
    app = this;

    setBoard(app.state.board);
    var cdcmd = app.state.cwd ? 'cd ' + app.state.cwd : 'cd .';
    runCommand(cdcmd, true);

    axios.get(tsURL('/sysinfo')).then(function(res) {
      var result = res.data.result;
      var home = result.os === 'windows' ? 'C:/mos' : result.home;
      app.setState({version: result.version, home: home});
      app.state.serial[0] += app.state.version + '\n\n';
    });

    axios.post(tsURL('/serial'), 'port=' + app.state.port);
    checkPorts();

    this.ws = util.wsconnect('/ws');
    this.ws.onmessage = function(msg) {
      var arr =
          msg.cmd.match(/uart|port/) ? app.state.serial : app.state.messages;
      var el = msg.cmd.match(/uart|port/) ? app.serialWindow : app.logWindow;
      if (msg.cmd === 'portchange') {
        msg.data = h('span', {class: 'text-danger'}, 'Serial ports changed!\n');
        checkPorts();
      }
      arr.push(msg.data);
      app.setState(app.state);
      scrollToBottom(el);
    }
  },
  componentWillUpdate: function(nextProps, nextState) {
    if (app.state.port !== nextState.port) {
      axios.post(tsURL('/serial'), 'port=' + nextState.port).then(function() {
        app.ls.mos_port = nextState.port;
      });
    }
    var truncateList = function(arr, max) {
      if (arr.length > max) arr.splice(0, arr.length - max);
    };
    truncateList(nextState.messages, maxLogMessages);
    truncateList(nextState.serial, maxLogMessages);
  },
  componentDidMount: function() {
    var resize = function() {
      var hBody = window.innerHeight;
      var hHeader = app.elHeader.offsetHeight;
      var hFooter = app.elFooter.offsetHeight;
      app.setState({mwh: hBody - (hHeader + hFooter) * 1.5});
    };
    resize();
    window.onresize = resize;
  },
  onKeyDown: function(ev) {
    ev.stopPropagation();
    var key = ev.key || ev.keyIdentifier;
    if (key.match(/^U\+/i))  // safari
      key = String.fromCharCode(ev.keyCode).toLowerCase();
    var s = shortcuts[key];
    if (s && ev.ctrlKey) {
      var cmd = s.cmd;
      ev.preventDefault();
      ev.target.value = cmd + ' ';
      if (s.go) {
        runCommand(cmd);
        focusCommandInput();
      } else {
        app.setState({command: cmd});
      }
    }
    if (key === 'Enter') {
      if (!app.state.showHistory) runCommand(app.state.command);
      app.setState({showHistory: false});
    }
    if (key === 'ArrowUp' || key == 'Up' || key === 'ArrowDown' ||
        key === 'Down') {
      var pos = (key === 'ArrowUp' || key === 'Up') ? ++app.state.historyPos :
                                                      --app.state.historyPos;
      var n = app.state.history.length;
      if (pos < 0) pos = 0;
      if (pos > n) pos = n;
      app.setState({command: app.state.history[n - pos], historyPos: pos});
      focusCommandInput();
      if (!app.state.showHistory) {
        app.setState({showHistory: true});
        setTimeout(function() {
          scrollToBottomIgnoreAutoscroll(app.historyWindow);
        }, 1);
      }
      setTimeout(function() {
        var pos = app.state.history.length - app.state.historyPos;
        var el = app.historyWindow ? app.historyWindow.childNodes[pos] : null;
        if (el && el.scrollIntoView) el.scrollIntoView();
      }, 1);
    }
    if (key === 'Escape' || ev.keyCode === 27) {
      app.setState({showHistory: false});
    }
  },
  render: function(props, state) {
    var winStyle =
        'height: ' + app.state.mwh + 'px; max-height: ' + app.state.mwh + 'px;';
    var wrap = app.state.wrap ? ' prewrap' : ' oa'
    var cls = 'border rounded bg-light w-100 h-100 px-2 my-0 vat' + wrap;
    onscroll = function(ev) {
      if (ev.target !== document) app.setState({autoscroll: false});
    };
    return h(
        'table', {
          class: 'w-100 h-100 mw-100 tlf position-relative',
          onKeyDown: app.onKeyDown,
          tabIndex: 0,
        },
        h('tr', {class: ''}, h('td', {colspan: 2, class: 'p-2'}, h(Header))),
        h('tr', {style: winStyle, ref: mkref('elMain'), class: ''},
          h('td', {class: 'pl-2'},
            h('pre', {
              class: cls + ' mr-2',
              style: winStyle,
              ref: mkref('logWindow'),
              onWheel: onscroll,
            },
              h('div', {class: 'my-2'}, app.state.messages))),
          h('td', {class: 'pr-3'},
            h('span', {
              class: 'badge badge-secondary position-absolute mt-1 px-2',
              style: 'z-index: 99; right: 1.5em; cursor: pointer;',
              onClick: function() {
                app.setState({autoscroll: true});
                scrollToBottom(app.logWindow);
                scrollToBottom(app.serialWindow);
              },
            },
              'Autoscroll: ' +
                  (app.state.autoscroll ? 'on' : 'off. Click here to enable')),
            h('pre', {
              class: cls + ' ml-2 text-muted',
              style: winStyle,
              ref: mkref('serialWindow'),
              onWheel: onscroll,
            },
              h('div', {class: 'my-2'}, app.state.serial)))),
        h('tr', {}, h('td', {colspan: 2, class: 'p-2'}, h(Prompt))));
  },
});

preact.render(h(App), document.body);
