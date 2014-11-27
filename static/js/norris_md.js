function addMenuItem(item, parent) {
	var parentElem, template;
	if (parent) {
		parentElem = $('#norris_md-nav .navbar-nav a[href="#' + parent.Path + '"]').siblings('ul.dropdown-menu');
	} else {
		parentElem = $('#norris_md-nav .navbar-nav');
	}
	if (item.NodeType == 'file') {
		template = $('#norris_md-template-menu-file').html();
		parentElem.append(Mustache.render(template, item));
	}
	if (item.NodeType == 'dir') {
		template = $('#norris_md-template-menu-dir').html();
		parentElem.append(Mustache.render(template, item));
		item.Children.forEach(function(child) { addMenuItem(child, item); });
	}
}

function buildNavigation() {
	var onSuccess = function(root, textStatus, jqXHR) {
		root.Children.forEach(function(child) { addMenuItem(child, undefined) });
	}
	var onError = function(jqXHR, textStatus, errorThrown) {
		console.log(jqXHR, textStatus, errorThrown);
	}
	$.ajax("norris_md/tree.json", {
		accepts: "application/json",
		success: onSuccess,
		error: onError
	});
}

function onCreated(evt) {
	var splitPath = evt.Path.split("/");
	if (splitPath.length > 2) {
		window.alert('Received an update for a newly created content document. Unfortunately the path ' + evt.Path + ' contains more than one level of folder hierarchy which is currently not supported by NorrisMd');
		return;
	}
	if (splitPath.length == 1) {
		addMenuItem(evt.NodeInfo, undefined);
	} else {
		addMenuItem(evt.NodeInfo, {Path:splitPath[0]});
	}
}

function onUpdated(evt) {

	// re-render page if we're currently displaying the udpated site
	if (window.location.hash.substr(1) == evt.Path) {
		loadPage(evt.Path);
	}
	console.log(event.data);
}

function onDeleted(evt) {
	var splitPath = evt.Path.split("/");
	if (splitPath.length > 2) {
		window.alert('Received an update for a newly created content document. Unfortunately the path ' + evt.Path + ' contains more than one level of folder hierarchy which is currently not supported by NorrisMd');
		return;
	}

	// remove menu item (if directory is removed it will also remove all children from navigation)
	$('#norris_md-nav a[href="#' + evt.Path + '"]').parent('li').remove();
	
	// if we're currently displaying one of the removed pages we shall navigate home
	if (evt.Path.indexOf(window.location.hash.substr(1)) > -1) {
		window.location.hash = "#";
	}
}

function listenForUpdates() {
	var socket = new WebSocket("ws://" + document.location.host + "/norris_md/ws");
	socket.onmessage = function (event) {
		var evt = JSON.parse(event.data);
		switch (evt.Type) {
			case 'CREATED':
				onCreated(evt);
				break;
			case 'UPDATED':
				onUpdated(evt);
				break;
			case 'DELETED':
				onDeleted(evt);
				break;
		}
	}
}

function toggleMenuLocation() {

	var path = window.location.hash.substr(1);

	$("#norris_md-nav").find('li').toggleClass('active', false);

	$("#norris_md-nav")
		.find("li a[href='#" + path + "']")
		.closest('li.dropdown')
		.toggleClass('active', true);
	
	$("#norris_md-nav")
		.find("li a[href='#" + path + "']")
		.closest('li')
		.toggleClass('active', true);
}

function loadPage(path) {
	$.ajax(path, {
		success: function(html) {
			$("#norris_md-content").empty();
			$("#norris_md-content").html(html);
		},
		error: function(jqXHR, textStatus, errorThrown) {
			html = $('<div/>')
				.append('<h2>jqXHR</h2>')
				.append('<pre>' + JSON.stringify(jqXHR, '', '  ') + '</pre>')
				.append('<h2>textStatus</h2>')
				.append('<pre>' + textStatus + '</pre>')
				.append('<h2>errorThrown</h2>')
				.append('<pre>' + errorThrown + '</pre>');
			$("#norris_md-content").empty();
			$("#norris_md-content").html(html);
		}
	});
}

$(function() {
	try {
		buildNavigation();
		listenForUpdates();
		var hash = window.location.hash.substr(1);
		if ("" != hash) {
			loadPage(hash);
		} else {
			window.onhashchange();
		}
	} catch (err) {
		console.log(err);
	}
});

window.onhashchange = function() {
	var target = window.location.hash.substr(1);
	if (target == '') {
		window.location.hash = '#Home.md';
		target = 'Home.md';
	}
	loadPage(target);
	toggleMenuLocation();
};
