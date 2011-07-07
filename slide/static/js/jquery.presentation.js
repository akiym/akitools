/*
 * Presentation Plugin
 * http://www.viget.com/
 *
 * Copyright (c) 2010 Trevor Davis
 * Dual licensed under the MIT and GPL licenses.
 * Uses the same license as jQuery, see:
 * http://jquery.org/license
 *
 * @version 0.2
 *
 * Example usage:
 * $('#slides').presentation({
 *   slide: '.slide',
 *   pagerClass: 'nav-pager',
 *   prevNextClass: 'nav-prev-next',
 *   prevText: 'Previous',
 *   nextText: 'Next',
 *   transition: 'fade'
 * });
 */
(function(c){c.fn.presentation=function(e){var f={slide:".slide",pagerClass:"nav-pager",prevNextClass:"nav-prev-next",prevText:"Previous",nextText:"Next"};c(this).each(function(){var a=c(this);a.count=1;a.changeSlide=function(b){a.slides.filter(":visible").hide();a.slides.filter(":nth-child("+b+")").show();a.find("."+a.options.pagerClass).children(".current").removeClass("current");a.find("."+a.options.pagerClass).children(":nth-child("+b+")").addClass("current");window.location.hash="#"+a.count};
a.pageClick=function(b){if(!b.parent().hasClass("current"))a.changeSlide(b.parent().prevAll().length+1),a.count=b.parent().prevAll().length+1};a.prev=function(){a.count>1&&a.count--;a.changeSlide(a.count)};a.next=function(){a.count<a.slides.length&&a.count++;a.changeSlide(a.count)};a.addControls=function(){a.numSlides=a.slides.length;for(var b='<ol class="'+a.options.pagerClass+'">',d=1;d<a.numSlides+1;d++)b+='<li><a href="#'+d+'">'+d+"</a></li>";a.append(b);a.currentHash?(a.find("."+a.options.pagerClass).children(":nth-child("+
a.currentHash+")").addClass("current"),a.count=a.currentHash):(a.find("."+a.options.pagerClass).children(":first-child").addClass("current"),a.count=1);a.append('<ul class="'+a.options.prevNextClass+'"><li><a href="#prev" class="prev">'+a.options.prevText+'</a></li><li><a href="#next" class="next">'+a.options.nextText+"</a></li>");a.find("."+a.options.pagerClass).find("a").click(function(){a.pageClick(c(this))});a.find("."+a.options.prevNextClass).find("a").click(function(){c(this).attr("class")===
"prev"?a.prev():c(this).attr("class")==="next"&&a.next();return!1});c(document).keyup(function(b){switch(b.keyCode){case 37:case 75:a.prev();break;case 39:case 74:a.next()}})};a.init=function(){a.options=c.extend(f,e);a.slides=a.find(a.options.slide);a.currentHash=window.location.hash.substr(1);a.currentHash?a.slides.filter(":not(:nth-child("+a.currentHash+"))").hide():a.slides.filter(":not(:first)").hide();a.addControls()};a.init()})}})(jQuery);